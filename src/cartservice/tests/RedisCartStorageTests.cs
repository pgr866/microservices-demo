using System;
using System.Threading.Tasks;
using cartservice.cartstore;
using StackExchange.Redis;
using Xunit;

namespace cartservice.tests;

public class RedisCartStorageTests
{
    [Fact]
    public async Task FailedConnectionIsRetriedOnNextCall()
    {
        var attempts = 0;
        using var storage = new RedisCartStorage(() =>
        {
            attempts++;
            return Task.FromException<IConnectionMultiplexer>(new RedisConnectionException(ConnectionFailureType.UnableToConnect, "down"));
        });

        await Assert.ThrowsAsync<RedisConnectionException>(() => storage.DeleteAsync("cart:u"));
        await Assert.ThrowsAsync<RedisConnectionException>(() => storage.DeleteAsync("cart:u"));

        Assert.Equal(2, attempts);
    }

    [Fact]
    public async Task PendingConnectionIsShared()
    {
        var attempts = 0;
        var pending = new TaskCompletionSource<IConnectionMultiplexer>();
        using var storage = new RedisCartStorage(() =>
        {
            attempts++;
            return pending.Task;
        });

        var first = storage.DeleteAsync("cart:u");
        var second = storage.DeleteAsync("cart:u");
        pending.SetException(new RedisConnectionException(ConnectionFailureType.UnableToConnect, "down"));

        await Assert.ThrowsAsync<RedisConnectionException>(() => first);
        await Assert.ThrowsAsync<RedisConnectionException>(() => second);
        Assert.Equal(1, attempts);
    }
}
