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

using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Console;
using cartservice;
using cartservice.logging;

CreateHostBuilder(args).Build().Run();

static IHostBuilder CreateHostBuilder(string[] args) =>
    Host.CreateDefaultBuilder(args)
        // JSON logs, like the other services.
        .ConfigureLogging(logging =>
        {
            logging.ClearProviders();
            logging.AddConsole(options => options.FormatterName = JsonLogFormatter.FormatterName);
            // Registered directly: AddConsoleFormatter<,> binds options from configuration
            // through reflection, unsafe in the trimmed image, and this formatter has none.
            logging.Services.TryAddEnumerable(ServiceDescriptor.Singleton<ConsoleFormatter, JsonLogFormatter>());
        })
        .ConfigureWebHostDefaults(webBuilder =>
        {
            webBuilder.UseStartup<Startup>();
        });