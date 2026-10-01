using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Extensions.Logging;

namespace cartservice.cartstore
{
    /// <summary>
    /// Keeps the carts in the process, when there is no Redis: lost on restart and not shared
    /// between replicas.
    /// </summary>
    public class InMemoryCartStorage : ICartStorage
    {
        private sealed class Cart
        {
            public OrderedDictionary<string, long> Items { get; } = new();
            public DateTimeOffset ExpiresAt { get; set; }
        }

        private readonly Dictionary<string, Cart> _carts = new();
        private readonly Lock _lock = new();
        private readonly TimeProvider _time;

        public InMemoryCartStorage(ILogger<InMemoryCartStorage> logger) : this(logger, TimeProvider.System)
        {
        }

        public InMemoryCartStorage(ILogger<InMemoryCartStorage> logger, TimeProvider time)
        {
            _time = time;
            logger.LogWarning("REDIS_ADDR not set: carts are kept in memory");
        }

        public Task IncrementAsync(string key, string field, long quantity, TimeSpan expiry)
        {
            lock (_lock)
            {
                var now = _time.GetUtcNow();
                // Like Redis, drop the expired carts instead of keeping them forever.
                foreach (var expired in _carts.Where(entry => entry.Value.ExpiresAt <= now).Select(entry => entry.Key).ToList())
                {
                    _carts.Remove(expired);
                }
                if (!_carts.TryGetValue(key, out var cart))
                {
                    _carts[key] = cart = new Cart();
                }
                cart.Items[field] = cart.Items.GetValueOrDefault(field) + quantity;
                cart.ExpiresAt = now + expiry;
            }
            return Task.CompletedTask;
        }

        public Task<IReadOnlyList<KeyValuePair<string, long>>> GetAllAsync(string key)
        {
            lock (_lock)
            {
                IReadOnlyList<KeyValuePair<string, long>> items =
                    _carts.TryGetValue(key, out var cart) && cart.ExpiresAt > _time.GetUtcNow() ? cart.Items.ToList() : [];
                return Task.FromResult(items);
            }
        }

        public Task DeleteAsync(string key)
        {
            lock (_lock)
            {
                _carts.Remove(key);
            }
            return Task.CompletedTask;
        }
    }
}
