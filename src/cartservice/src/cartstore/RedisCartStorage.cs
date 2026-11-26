using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using StackExchange.Redis;

namespace cartservice.cartstore;

/// <summary>Each cart is a Redis hash: HINCRBY adds to a product's quantity atomically.</summary>
public sealed class RedisCartStorage : ICartStorage, IDisposable
{
    private readonly Func<Task<IConnectionMultiplexer>> _connect;
    private readonly object _connectionLock = new();
    private Task<IConnectionMultiplexer> _connection;

    public RedisCartStorage(string configuration)
        : this(Connector(configuration))
    {
    }

    public RedisCartStorage(Func<Task<IConnectionMultiplexer>> connect)
    {
        _connect = connect;
    }

    private static Func<Task<IConnectionMultiplexer>> Connector(string configuration)
    {
        var options = ConfigurationOptions.Parse(configuration);
        // Keep retrying in the background instead of failing while Redis is unreachable.
        options.AbortOnConnectFail = false;
        // While disconnected, fail each call at once (Unavailable) instead of queueing it until its deadline.
        options.BacklogPolicy = BacklogPolicy.FailFast;
        // Connect on first use, so the service starts even before Redis does.
        return async () => await ConnectionMultiplexer.ConnectAsync(options);
    }

    /// <summary>
    /// Connects on first use and then reuses the connection. A failed attempt is not cached
    /// (unlike Lazy&lt;Task&gt;): the next call tries again instead of failing forever.
    /// </summary>
    private Task<IConnectionMultiplexer> ConnectionAsync()
    {
        lock (_connectionLock)
        {
            if (_connection is null || _connection.IsFaulted || _connection.IsCanceled)
            {
                _connection = _connect();
            }
            return _connection;
        }
    }

    private async Task<IDatabase> DatabaseAsync() => (await ConnectionAsync()).GetDatabase();

    public async Task IncrementAsync(string key, string field, long quantity, TimeSpan expiry)
    {
        // One transaction, so a cart never ends up without an expiry.
        var transaction = (await DatabaseAsync()).CreateTransaction();
        _ = transaction.HashIncrementAsync(key, field, quantity);
        _ = transaction.KeyExpireAsync(key, expiry);
        await transaction.ExecuteAsync();
    }

    public async Task<IReadOnlyList<KeyValuePair<string, long>>> GetAllAsync(string key)
    {
        var entries = await (await DatabaseAsync()).HashGetAllAsync(key);
        return [.. entries.Select(entry => KeyValuePair.Create(entry.Name.ToString(), (long)entry.Value))];
    }

    public async Task DeleteAsync(string key)
    {
        await (await DatabaseAsync()).KeyDeleteAsync(key);
    }

    public void Dispose()
    {
        Task<IConnectionMultiplexer> connection;
        lock (_connectionLock)
        {
            connection = _connection;
        }
        if (connection is { IsCompletedSuccessfully: true })
        {
            connection.Result.Dispose();
        }
    }
}
