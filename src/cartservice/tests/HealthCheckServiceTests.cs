using System.Threading;
using System.Threading.Tasks;
using Grpc.Health.V1;
using Microsoft.Extensions.Hosting;
using cartservice.services;
using Xunit;

namespace cartservice.tests;

public class HealthCheckServiceTests
{
    private sealed class FakeLifetime : IHostApplicationLifetime
    {
        private readonly CancellationTokenSource _stopping = new();

        public CancellationToken ApplicationStarted => CancellationToken.None;
        public CancellationToken ApplicationStopping => _stopping.Token;
        public CancellationToken ApplicationStopped => CancellationToken.None;
        public void StopApplication() => _stopping.Cancel();
    }

    [Fact]
    public async Task Check_WhileRunning_ReportsServing()
    {
        var service = new HealthCheckService(new FakeLifetime());

        var response = await service.Check(new HealthCheckRequest(), null);

        Assert.Equal(HealthCheckResponse.Types.ServingStatus.Serving, response.Status);
    }

    [Fact]
    public async Task Check_OnceShutdownStarts_ReportsNotServing()
    {
        var lifetime = new FakeLifetime();
        var service = new HealthCheckService(lifetime);

        lifetime.StopApplication();
        var response = await service.Check(new HealthCheckRequest(), null);

        Assert.Equal(HealthCheckResponse.Types.ServingStatus.NotServing, response.Status);
    }
}
