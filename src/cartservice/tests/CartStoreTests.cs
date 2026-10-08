using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using cartservice.cartstore;
using Grpc.Core;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace cartservice.tests
{
    public class CartStoreTests
    {
        private static CartStore NewStore(ICartStorage storage = null) =>
            new(storage ?? new InMemoryCartStorage(NullLogger<InMemoryCartStorage>.Instance), NullLogger<CartStore>.Instance);

        /// <summary>Keeps every message logged, with every level enabled (like LOG_LEVEL=debug).</summary>
        private sealed class RecordingLogger<T> : ILogger<T>
        {
            public List<(LogLevel Level, string Message)> Entries { get; } = [];
            public IDisposable BeginScope<TState>(TState state) where TState : notnull => null;
            public bool IsEnabled(LogLevel logLevel) => true;
            public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception exception, Func<TState, Exception, string> formatter) =>
                Entries.Add((logLevel, formatter(state, exception)));
        }

        /// <summary>A storage that is unreachable, like a Redis that went down.</summary>
        private class FailingStorage : ICartStorage
        {
            private static InvalidOperationException Down() => new("storage down");

            public Task IncrementAsync(string key, string field, long quantity, TimeSpan expiry) => throw Down();
            public Task<IReadOnlyList<KeyValuePair<string, long>>> GetAllAsync(string key) => throw Down();
            public Task DeleteAsync(string key) => throw Down();
        }

        [Fact]
        public async Task AddItem_DifferentProducts_KeepsOneLinePerProduct()
        {
            var store = NewStore();

            await store.AddItemAsync("user", "product-a", 1);
            await store.AddItemAsync("user", "product-b", 3);

            var cart = await store.GetCartAsync("user");
            Assert.Equal(2, cart.Items.Count);
            Assert.Equal(1, cart.Items[0].Quantity);
            Assert.Equal(3, cart.Items[1].Quantity);
        }

        [Fact]
        public async Task AddItem_ForOneUser_DoesNotTouchAnotherUsersCart()
        {
            var store = NewStore();

            await store.AddItemAsync("alice", "product-a", 1);

            Assert.Empty((await store.GetCartAsync("bob")).Items);
        }

        [Fact]
        public async Task Operations_LogAtDebugWithoutTheUserId()
        {
            // The user ID is the session ID and the cart's key: it must stay out of the logs.
            var logger = new RecordingLogger<CartStore>();
            var store = new CartStore(new InMemoryCartStorage(NullLogger<InMemoryCartStorage>.Instance), logger);

            await store.AddItemAsync("session-1234", "product-a", 2);
            await store.GetCartAsync("session-1234");
            await store.EmptyCartAsync("session-1234");

            Assert.Contains((LogLevel.Debug, "AddItem product_id=product-a quantity=2"), logger.Entries);
            Assert.All(logger.Entries, entry =>
            {
                Assert.Equal(LogLevel.Debug, entry.Level);
                Assert.DoesNotContain("session-1234", entry.Message);
            });
        }

        [Fact]
        public async Task AddItem_AfterEmptyCart_StartsFromScratch()
        {
            var store = NewStore();
            await store.AddItemAsync("user", "product-a", 5);
            await store.EmptyCartAsync("user");

            await store.AddItemAsync("user", "product-a", 1);

            var cart = await store.GetCartAsync("user");
            Assert.Equal(1, Assert.Single(cart.Items).Quantity);
        }

        [Fact]
        public async Task AddItem_Concurrently_CountsEveryAddition()
        {
            var store = NewStore();

            // Used to keep only some of them: each call read the cart, added its item and wrote the
            // whole cart back, overwriting what the others had written in between.
            await Task.WhenAll(Enumerable.Range(0, 100).Select(_ => Task.Run(() => store.AddItemAsync("user", "product-a", 1))));

            var cart = await store.GetCartAsync("user");
            Assert.Equal(100, Assert.Single(cart.Items).Quantity);
        }

        /// <summary>A clock the test moves by hand.</summary>
        private sealed class ManualTime : TimeProvider
        {
            public DateTimeOffset Now { get; set; } = new(2026, 10, 1, 12, 0, 0, TimeSpan.Zero);
            public override DateTimeOffset GetUtcNow() => Now;
        }

        [Fact]
        public async Task Cart_ExpiresAfterItsLifetimeSinceTheLastAddition()
        {
            var time = new ManualTime();
            var store = NewStore(new InMemoryCartStorage(NullLogger<InMemoryCartStorage>.Instance, time));
            await store.AddItemAsync("user", "product-a", 1);

            time.Now += CartStore.CartLifetime - TimeSpan.FromMinutes(1);
            await store.AddItemAsync("user", "product-a", 1);
            time.Now += CartStore.CartLifetime - TimeSpan.FromMinutes(1);
            Assert.Equal(2, Assert.Single((await store.GetCartAsync("user")).Items).Quantity);

            time.Now += TimeSpan.FromMinutes(2);
            Assert.Empty((await store.GetCartAsync("user")).Items);
        }

        [Fact]
        public async Task ExpiredCarts_AreDroppedFromMemory()
        {
            var time = new ManualTime();
            var storage = new InMemoryCartStorage(NullLogger<InMemoryCartStorage>.Instance, time);
            await storage.IncrementAsync("old", "product-a", 1, TimeSpan.FromHours(1));

            time.Now += TimeSpan.FromHours(2);
            await storage.IncrementAsync("new", "product-a", 1, TimeSpan.FromHours(1));

            // Even if it were read with a clock gone back, it is no longer there.
            time.Now -= TimeSpan.FromHours(2);
            Assert.Empty(await storage.GetAllAsync("old"));
        }

        [Fact]
        public async Task GetCart_SetsTheUserIdOnlyWhenTheCartHasItems()
        {
            var store = NewStore();
            Assert.Equal(new Hipstershop.Cart(), await store.GetCartAsync("user"));

            await store.AddItemAsync("user", "product-a", 1);

            Assert.Equal("user", (await store.GetCartAsync("user")).UserId);
        }

        [Fact]
        public async Task StorageUnavailable_EveryOperationFailsWithUnavailable()
        {
            var store = NewStore(new FailingStorage());

            var add = await Assert.ThrowsAsync<RpcException>(() => store.AddItemAsync("user", "product-a", 1));
            var get = await Assert.ThrowsAsync<RpcException>(() => store.GetCartAsync("user"));
            var empty = await Assert.ThrowsAsync<RpcException>(() => store.EmptyCartAsync("user"));

            Assert.All([add, get, empty], ex =>
            {
                Assert.Equal(StatusCode.Unavailable, ex.StatusCode);
                Assert.Equal("Can't access cart storage.", ex.Status.Detail);
            });
        }
    }
}
