using Microsoft.AspNetCore.Builder;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using cartservice.cartstore;
using cartservice.services;

namespace cartservice;

public class Startup
{
    public Startup(IConfiguration configuration)
    {
        Configuration = configuration;
    }

    public IConfiguration Configuration { get; }

    public void ConfigureServices(IServiceCollection services)
    {
        string redisAddress = Configuration["REDIS_ADDR"];

        if (!string.IsNullOrEmpty(redisAddress))
        {
            // A factory and not an instance, so the container disposes the Redis connection on shutdown.
            services.AddSingleton<ICartStorage>(_ => new RedisCartStorage(redisAddress));
        }
        else
        {
            services.AddSingleton<ICartStorage, InMemoryCartStorage>();
        }

        services.AddSingleton<ICartStore, CartStore>();
        services.AddGrpc();
    }

    public void Configure(IApplicationBuilder app)
    {
        app.UseRouting();

        app.UseEndpoints(endpoints =>
        {
            endpoints.MapGrpcService<CartService>();
            endpoints.MapGrpcService<HealthCheckService>();
        });
    }
}
