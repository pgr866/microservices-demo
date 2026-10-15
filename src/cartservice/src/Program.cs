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
using System.Collections.Generic;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Console;
using cartservice;
using cartservice.logging;

LogLevel logLevel;
try
{
    logLevel = LogLevelSetting.Parse(Environment.GetEnvironmentVariable("LOG_LEVEL"));
}
catch (ArgumentException e)
{
    // Before the host exists, so written by hand: one JSON line instead of a stack trace.
    Console.Error.WriteLine(JsonLogFormatter.FormatLine(LogLevel.Error, "cartservice", e.Message));
    return 1;
}

CreateHostBuilder(args, logLevel).Build().Run();
return 0;

static IHostBuilder CreateHostBuilder(string[] args, LogLevel logLevel) =>
    Host.CreateDefaultBuilder(args)
        // Added last, so LOG_LEVEL overrides the level in appsettings.json.
        .ConfigureAppConfiguration(config => config.AddInMemoryCollection(new Dictionary<string, string>
        {
            ["Logging:LogLevel:Default"] = logLevel.ToString(),
        }))
        .ConfigureLogging(logging =>
        {
            logging.ClearProviders();
            logging.AddConsole(options => options.FormatterName = JsonLogFormatter.FormatterName);
            // Registered directly: AddConsoleFormatter<,> binds options from configuration
            // through reflection, unsafe in the trimmed image, and this formatter has none.
            logging.Services.TryAddEnumerable(ServiceDescriptor.Singleton<ConsoleFormatter, JsonLogFormatter>());
        })
        // In-flight calls get 10 s to finish on SIGTERM, less than the 30 s Kubernetes waits.
        .ConfigureServices(services => services.Configure<HostOptions>(options => options.ShutdownTimeout = TimeSpan.FromSeconds(10)))
        .ConfigureWebHostDefaults(webBuilder =>
        {
            webBuilder.UseStartup<Startup>();
        });
