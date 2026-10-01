// Copyright 2020 Google LLC
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
using System.Threading.Tasks;
using Grpc.Core;
using Microsoft.Extensions.Logging;
using cartservice.cartstore;
using Hipstershop;

namespace cartservice.services
{
    public class CartService : Hipstershop.CartService.CartServiceBase
    {
        private readonly static Empty Empty = new Empty();
        private readonly ICartStore _cartStore;

        public CartService(ICartStore cartStore)
        {
            _cartStore = cartStore;
        }

        public async override Task<Empty> AddItem(AddItemRequest request, ServerCallContext context)
        {
            RequireUserId(request.UserId);
            // An empty product ID would be stored, and then every checkout of
            // the cart would fail because the catalog has no such product.
            if (string.IsNullOrWhiteSpace(request.Item?.ProductId))
            {
                throw new RpcException(new Status(StatusCode.InvalidArgument, "Item product ID is required"));
            }
            if (request.Item.Quantity < 1)
            {
                throw new RpcException(new Status(StatusCode.InvalidArgument, "Item quantity must be at least 1"));
            }
            await _cartStore.AddItemAsync(request.UserId, request.Item.ProductId, request.Item.Quantity);
            return Empty;
        }

        public override Task<Cart> GetCart(GetCartRequest request, ServerCallContext context)
        {
            RequireUserId(request.UserId);
            return _cartStore.GetCartAsync(request.UserId);
        }

        public async override Task<Empty> EmptyCart(EmptyCartRequest request, ServerCallContext context)
        {
            RequireUserId (request.UserId);
            await _cartStore.EmptyCartAsync(request.UserId);
            return Empty;
        }

        // The user ID is the cart's storage key: without this check, every call
        // missing it would share the same cart.
        private static void RequireUserId(string userId)
        {
            if (string.IsNullOrWhiteSpace(userId))
            {
                throw new RpcException(new Status(StatusCode.InvalidArgument, "User ID is required"));
            }
        }
    }
}