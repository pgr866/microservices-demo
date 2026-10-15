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

using System.Threading.Tasks;
using Grpc.Core;
using Grpc.Health.V1;
using Microsoft.Extensions.Hosting;
using static Grpc.Health.V1.Health;

namespace cartservice.services;

/// <summary>
/// SERVING regardless of Redis, so a Redis outage does not restart every pod; NOT_SERVING once
/// shutdown starts, so the probes take the pod out while it drains its in-flight calls.
/// </summary>
public class HealthCheckService : HealthBase
{
    private readonly IHostApplicationLifetime _lifetime;

    public HealthCheckService(IHostApplicationLifetime lifetime)
    {
        _lifetime = lifetime;
    }

    public override Task<HealthCheckResponse> Check(HealthCheckRequest request, ServerCallContext context)
    {
        return Task.FromResult(new HealthCheckResponse
        {
            Status = _lifetime.ApplicationStopping.IsCancellationRequested
                ? HealthCheckResponse.Types.ServingStatus.NotServing
                : HealthCheckResponse.Types.ServingStatus.Serving
        });
    }
}
