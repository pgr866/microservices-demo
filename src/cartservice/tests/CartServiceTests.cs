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
using System.Threading;
using System.Threading.Tasks;
using Grpc.Core;
using Grpc.Net.Client;
using Hipstershop;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.Hosting;
using Xunit;
using static Hipstershop.CartService;

namespace cartservice.tests;

public class CartServiceTests
{
    private static async Task<(IHost Host, CartServiceClient Client)> StartAsync()
    {
        var host = await new HostBuilder()
            .ConfigureWebHost(webBuilder => webBuilder.UseStartup<Startup>().UseTestServer())
            .StartAsync(TestContext.Current.CancellationToken);
        var httpClient = host.GetTestClient();
        var channel = GrpcChannel.ForAddress(httpClient.BaseAddress, new GrpcChannelOptions { HttpClient = httpClient });
        return (host, new CartServiceClient(channel));
    }

    private static AddItemRequest Add(string userId, string productId, int quantity) =>
        new() { UserId = userId, Item = new CartItem { ProductId = productId, Quantity = quantity } };

    private static async Task<StatusCode> StatusOf(Func<Task> call) =>
        (await Assert.ThrowsAsync<RpcException>(call)).StatusCode;

    private static CancellationToken Ct => TestContext.Current.CancellationToken;

    [Fact]
    public async Task GetCart_UnknownUser_ReturnsAnEmptyCart()
    {
        var (host, client) = await StartAsync();
        using var _ = host;

        Assert.Equal(new Cart(), await client.GetCartAsync(new GetCartRequest { UserId = Guid.NewGuid().ToString() }, cancellationToken: Ct));
    }

    [Fact]
    public async Task AddItem_SameProductTwice_AddsUpTheQuantities()
    {
        var (host, client) = await StartAsync();
        using var _ = host;

        await client.AddItemAsync(Add("user", "product-a", 1), cancellationToken: Ct);
        await client.AddItemAsync(Add("user", "product-a", 2), cancellationToken: Ct);

        var cart = await client.GetCartAsync(new GetCartRequest { UserId = "user" }, cancellationToken: Ct);
        Assert.Equal("user", cart.UserId);
        var item = Assert.Single(cart.Items);
        Assert.Equal("product-a", item.ProductId);
        Assert.Equal(3, item.Quantity);
    }

    [Fact]
    public async Task EmptyCart_RemovesEveryItemOfThatUserOnly()
    {
        var (host, client) = await StartAsync();
        using var _ = host;
        await client.AddItemAsync(Add("alice", "product-a", 1), cancellationToken: Ct);
        await client.AddItemAsync(Add("alice", "product-b", 1), cancellationToken: Ct);
        await client.AddItemAsync(Add("bob", "product-a", 1), cancellationToken: Ct);

        await client.EmptyCartAsync(new EmptyCartRequest { UserId = "alice" }, cancellationToken: Ct);

        Assert.Empty((await client.GetCartAsync(new GetCartRequest { UserId = "alice" }, cancellationToken: Ct)).Items);
        Assert.Single((await client.GetCartAsync(new GetCartRequest { UserId = "bob" }, cancellationToken: Ct)).Items);
    }

    [Fact]
    public async Task EmptyCart_UnknownUser_Succeeds()
    {
        var (host, client) = await StartAsync();
        using var _ = host;

        await client.EmptyCartAsync(new EmptyCartRequest { UserId = "nobody" }, cancellationToken: Ct);
    }

    [Theory]
    [InlineData("user", "product-a", 0)]
    [InlineData("user", "product-a", -1)]
    [InlineData("", "product-a", 1)]
    [InlineData(" ", "product-a", 1)]
    [InlineData("user", "", 1)]
    [InlineData("user", " ", 1)]
    public async Task AddItem_InvalidRequest_InvalidArgumentAndNothingStored(string userId, string productId, int quantity)
    {
        var (host, client) = await StartAsync();
        using var _ = host;

        Assert.Equal(StatusCode.InvalidArgument,
            await StatusOf(() => client.AddItemAsync(Add(userId, productId, quantity), cancellationToken: Ct).ResponseAsync));
        if (!string.IsNullOrWhiteSpace(userId))
        {
            Assert.Empty((await client.GetCartAsync(new GetCartRequest { UserId = userId }, cancellationToken: Ct)).Items);
        }
    }

    [Fact]
    public async Task AddItem_NoItem_InvalidArgument()
    {
        var (host, client) = await StartAsync();
        using var _ = host;

        Assert.Equal(StatusCode.InvalidArgument,
            await StatusOf(() => client.AddItemAsync(new AddItemRequest { UserId = "user" }, cancellationToken: Ct).ResponseAsync));
    }

    [Theory]
    [InlineData("")]
    [InlineData("  ")]
    public async Task GetCartAndEmptyCart_NoUserId_InvalidArgument(string userId)
    {
        var (host, client) = await StartAsync();
        using var _ = host;

        Assert.Equal(StatusCode.InvalidArgument,
            await StatusOf(() => client.GetCartAsync(new GetCartRequest { UserId = userId }, cancellationToken: Ct).ResponseAsync));
        Assert.Equal(StatusCode.InvalidArgument,
            await StatusOf(() => client.EmptyCartAsync(new EmptyCartRequest { UserId = userId }, cancellationToken: Ct).ResponseAsync));
    }
}
