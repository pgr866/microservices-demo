using System;
using Microsoft.Extensions.Logging;

namespace cartservice.logging
{
    /// <summary>
    /// Reads LOG_LEVEL with the same values as the other services: debug, info (the default),
    /// warn or error.
    /// </summary>
    public static class LogLevelSetting
    {
        public static LogLevel Parse(string value) => value?.ToLowerInvariant() switch
        {
            null or "" or "info" => LogLevel.Information,
            "debug" => LogLevel.Debug,
            "warn" => LogLevel.Warning,
            "error" => LogLevel.Error,
            _ => throw new ArgumentException($"invalid LOG_LEVEL \"{value}\": want debug, info, warn or error"),
        };
    }
}
