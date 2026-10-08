using System.Collections.Generic;
using System.Threading.Tasks;
using Grpc.Health.V1;
using Grpc.Net.Client;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.TestHost;
using cartservice.cartstore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Xunit;

namespace cartservice.tests
{
    public class StartupTests
    {
        private static Task<IHost> StartHost(Dictionary<string, string> config = null) =>
            new HostBuilder()
                .ConfigureAppConfiguration(c => c.AddInMemoryCollection(config ?? []))
                .ConfigureWebHost(webBuilder => webBuilder.UseStartup<Startup>().UseTestServer())
                .StartAsync(TestContext.Current.CancellationToken);

        [Fact]
        public async Task WithoutRedisAddr_UsesInMemoryStorage()
        {
            using var host = await StartHost();

            Assert.IsType<InMemoryCartStorage>(host.Services.GetRequiredService<ICartStorage>());
        }

        [Fact]
        public async Task WithRedisAddr_UsesRedisStorage()
        {
            // RedisCartStorage connects on first use, so resolving it needs no running Redis.
            using var host = await StartHost(new Dictionary<string, string> { ["REDIS_ADDR"] = "redis-cart:6379" });

            Assert.IsType<RedisCartStorage>(host.Services.GetRequiredService<ICartStorage>());
        }

        [Fact]
        public async Task HealthCheck_ReportsServing()
        {
            using var host = await StartHost();
            var httpClient = host.GetTestClient();
            var channel = GrpcChannel.ForAddress(httpClient.BaseAddress, new GrpcChannelOptions { HttpClient = httpClient });

            var response = await new Health.HealthClient(channel).CheckAsync(
                new HealthCheckRequest(), cancellationToken: TestContext.Current.CancellationToken);

            Assert.Equal(HealthCheckResponse.Types.ServingStatus.Serving, response.Status);
        }
    }
}
