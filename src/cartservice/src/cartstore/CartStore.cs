// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

using System;
using System.Linq;
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Extensions.Logging;

namespace cartservice.cartstore;

public class CartStore : ICartStore
{
    private readonly ICartStorage _storage;
    private readonly ILogger<CartStore> _logger;

    public CartStore(ICartStorage storage, ILogger<CartStore> logger)
    {
        _storage = storage;
        _logger = logger;
    }

    // frontend's cookie lifetime: after that the cart is unreachable and would only take up memory.
    public static readonly TimeSpan CartLifetime = TimeSpan.FromHours(48);

    private static string Key(string userId) => "cart:" + userId;

    public Task AddItemAsync(string userId, string productId, int quantity)
    {
        // Debug is off in production: skip building the arguments.
        if (_logger.IsEnabled(LogLevel.Debug))
        {
            _logger.LogDebug("AddItem product_id={ProductId} quantity={Quantity}", productId, quantity);
        }
        // One atomic step in the storage, so concurrent additions all count.
        return Guard(() => _storage.IncrementAsync(Key(userId), productId, quantity, CartLifetime));
    }

    public Task EmptyCartAsync(string userId)
    {
        _logger.LogDebug("EmptyCart");
        return Guard(() => _storage.DeleteAsync(Key(userId)));
    }

    public async Task<Hipstershop.Cart> GetCartAsync(string userId)
    {
        _logger.LogDebug("GetCart");
        var items = await Guard(() => _storage.GetAllAsync(Key(userId)));

        // No cart yet is not an error: it is an empty cart, without a user ID.
        var cart = new Hipstershop.Cart();
        if (items.Count == 0)
        {
            return cart;
        }
        cart.UserId = userId;
        cart.Items.AddRange(items.Select(item => new Hipstershop.CartItem
        {
            ProductId = item.Key,
            Quantity = (int)Math.Min(item.Value, int.MaxValue)
        }));
        return cart;
    }

    private async Task Guard(Func<Task> operation)
    {
        await Guard(async () =>
        {
            await operation();
            return true;
        });
    }

    // Unavailable: a storage outage is transient. The exception stays in the log, not in the status.
    private async Task<T> Guard<T>(Func<Task<T>> operation)
    {
        try
        {
            return await operation();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Can't access cart storage");
            throw new RpcException(new Status(StatusCode.Unavailable, "Can't access cart storage."));
        }
    }
}
