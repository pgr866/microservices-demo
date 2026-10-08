using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using StackExchange.Redis;

namespace cartservice.cartstore
{
    /// <summary>Each cart is a Redis hash: HINCRBY adds to a product's quantity atomically.</summary>
    public sealed class RedisCartStorage : ICartStorage, IDisposable
    {
        private readonly Lazy<Task<ConnectionMultiplexer>> _connection;

        public RedisCartStorage(string configuration)
        {
            var options = ConfigurationOptions.Parse(configuration);
            // Keep retrying in the background, instead of failing, while Redis isn't reachable.
            options.AbortOnConnectFail = false;
            // Connect on first use, so the service starts even before Redis does.
            _connection = new Lazy<Task<ConnectionMultiplexer>>(() => ConnectionMultiplexer.ConnectAsync(options));
        }

        private async Task<IDatabase> DatabaseAsync() => (await _connection.Value).GetDatabase();

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
            if (_connection.IsValueCreated && _connection.Value.IsCompletedSuccessfully)
            {
                _connection.Value.Result.Dispose();
            }
        }
    }
}
