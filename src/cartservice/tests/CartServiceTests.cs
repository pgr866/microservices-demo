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
using System.Threading.Tasks;
using Grpc.Core;
using Grpc.Net.Client;
using Hipstershop;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.Hosting;
using Xunit;
using static Hipstershop.CartService;

namespace cartservice.tests
{
    public class CartServiceTests
    {
        private readonly IHostBuilder _host;

        public CartServiceTests()
        {
            _host = new HostBuilder().ConfigureWebHost(webBuilder =>
            {
                webBuilder
                    .UseStartup<Startup>()
                    .UseTestServer();
            });
        }

        [Fact]
        public async Task GetItem_NoAddItemBefore_EmptyCartReturned()
        {
            using var server = await _host.StartAsync(TestContext.Current.CancellationToken);
            var httpClient = server.GetTestClient();

            string userId = Guid.NewGuid().ToString();

            var channel = GrpcChannel.ForAddress(httpClient.BaseAddress, new GrpcChannelOptions
            {
                HttpClient = httpClient
            });

            var cartClient = new CartServiceClient(channel);

            var request = new GetCartRequest
            {
                UserId = userId,
            };

            var cart = await cartClient.GetCartAsync(request, cancellationToken: TestContext.Current.CancellationToken);
            Assert.NotNull(cart);

            // All grpc objects implement IEquitable, so we can compare equality with by-value semantics
            Assert.Equal(new Cart(), cart);
        }

        [Fact]
        public async Task AddItem_ItemExists_Updated()
        {
            using var server = await _host.StartAsync(TestContext.Current.CancellationToken);
            var httpClient = server.GetTestClient();

            string userId = Guid.NewGuid().ToString();

            var channel = GrpcChannel.ForAddress(httpClient.BaseAddress, new GrpcChannelOptions
            {
                HttpClient = httpClient
            });

            var client = new CartServiceClient(channel);
            var request = new AddItemRequest
            {
                UserId = userId,
                Item = new CartItem
                {
                    ProductId = "1",
                    Quantity = 1
                }
            };

            // First add - nothing should fail
            await client.AddItemAsync(request, cancellationToken: TestContext.Current.CancellationToken);

            // Second add of existing product - quantity should be updated
            await client.AddItemAsync(request, cancellationToken: TestContext.Current.CancellationToken);

            var getCartRequest = new GetCartRequest
            {
                UserId = userId
            };
            var cart = await client.GetCartAsync(getCartRequest, cancellationToken: TestContext.Current.CancellationToken);
            Assert.NotNull(cart);
            Assert.Equal(userId, cart.UserId);
            Assert.Equal(2, Assert.Single(cart.Items).Quantity);

            await client.EmptyCartAsync(new EmptyCartRequest { UserId = userId }, cancellationToken: TestContext.Current.CancellationToken);
        }

        [Fact]
        public async Task AddItem_New_Inserted()
        {
            using var server = await _host.StartAsync(TestContext.Current.CancellationToken);
            var httpClient = server.GetTestClient();

            string userId = Guid.NewGuid().ToString();

            var channel = GrpcChannel.ForAddress(httpClient.BaseAddress, new GrpcChannelOptions
            {
                HttpClient = httpClient
            });

            var client = new CartServiceClient(channel);

            var request = new AddItemRequest
            {
                UserId = userId,
                Item = new CartItem
                {
                    ProductId = "1",
                    Quantity = 1
                }
            };

            await client.AddItemAsync(request, cancellationToken: TestContext.Current.CancellationToken);

            var getCartRequest = new GetCartRequest
            {
                UserId = userId
            };
            var cart = await client.GetCartAsync(getCartRequest, cancellationToken: TestContext.Current.CancellationToken);
            Assert.NotNull(cart);
            Assert.Equal(userId, cart.UserId);
            Assert.Single(cart.Items);

            await client.EmptyCartAsync(new EmptyCartRequest { UserId = userId }, cancellationToken: TestContext.Current.CancellationToken);
            cart = await client.GetCartAsync(getCartRequest, cancellationToken: TestContext.Current.CancellationToken);
            Assert.Empty(cart.Items);
        }

        [Theory]
        [InlineData("user", "1", 0)]
        [InlineData("user", "1", -1)]
        [InlineData("", "1", 1)]
        [InlineData(" ", "1", 1)]
        [InlineData("user", "", 1)]
        [InlineData("user", null, 1)]
        public async Task AddItem_InvalidItem_InvalidArgument(string userId, string productId, int quantity)
        {
            using var server = await _host.StartAsync(TestContext.Current.CancellationToken);
            var client = NewClient(server);

            var request = new AddItemRequest { UserId = userId, Item = new CartItem { Quantity = quantity } };
            if (productId != null)
            {
                request.Item.ProductId = productId;
            }

            var ex = await Assert.ThrowsAsync<RpcException>(() =>
                client.AddItemAsync(request, cancellationToken: TestContext.Current.CancellationToken).ResponseAsync);
            Assert.Equal(StatusCode.InvalidArgument, ex.StatusCode);
        }

        [Fact]
        public async Task AddItem_NoItem_InvalidArgument()
        {
            using var server = await _host.StartAsync(TestContext.Current.CancellationToken);
            var client = NewClient(server);

            var ex = await Assert.ThrowsAsync<RpcException>(() =>
                client.AddItemAsync(new AddItemRequest { UserId = "user" }, cancellationToken: TestContext.Current.CancellationToken).ResponseAsync);
            Assert.Equal(StatusCode.InvalidArgument, ex.StatusCode);
        }

        [Fact]
        public async Task GetCartAndEmptyCart_NoUserId_InvalidArgument()
        {
            using var server = await _host.StartAsync(TestContext.Current.CancellationToken);
            var client = NewClient(server);

            var get = await Assert.ThrowsAsync<RpcException>(() =>
                client.GetCartAsync(new GetCartRequest(), cancellationToken: TestContext.Current.CancellationToken).ResponseAsync);
            var empty = await Assert.ThrowsAsync<RpcException>(() =>
                client.EmptyCartAsync(new EmptyCartRequest(), cancellationToken: TestContext.Current.CancellationToken).ResponseAsync);

            Assert.Equal(StatusCode.InvalidArgument, get.StatusCode);
            Assert.Equal(StatusCode.InvalidArgument, empty.StatusCode);
        }

        private static CartServiceClient NewClient(IHost server)
        {
            var httpClient = server.GetTestClient();
            var channel = GrpcChannel.ForAddress(httpClient.BaseAddress, new GrpcChannelOptions
            {
                HttpClient = httpClient
            });
            return new CartServiceClient(channel);
        }
    }
}
