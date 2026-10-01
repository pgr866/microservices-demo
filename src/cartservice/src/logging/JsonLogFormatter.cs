using System;
using System.IO;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Microsoft.Extensions.Logging.Console;

namespace cartservice.logging
{
    /// <summary>
    /// Writes each log entry as one JSON line, with the same field names as the other services'
    /// logs (timestamp, severity, message), plus the logger and the exception if any.
    /// </summary>
    public sealed class JsonLogFormatter : ConsoleFormatter
    {
        public const string FormatterName = "cartservice-json";

        public JsonLogFormatter() : base(FormatterName)
        {
        }

        public override void Write<TState>(in LogEntry<TState> logEntry, IExternalScopeProvider scopeProvider, TextWriter textWriter)
        {
            var message = logEntry.Formatter?.Invoke(logEntry.State, logEntry.Exception);
            if (message is null && logEntry.Exception is null)
            {
                return;
            }

            // Utf8JsonWriter instead of JsonSerializer: the image is published trimmed, which
            // reflection-based serialization doesn't support.
            using var buffer = new MemoryStream();
            // Relaxed escaping keeps characters such as ' or + readable: the strict default only
            // matters for JSON embedded in HTML, and a log line never is.
            using (var json = new Utf8JsonWriter(buffer, new JsonWriterOptions { Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping }))
            {
                json.WriteStartObject();
                json.WriteString("timestamp", DateTimeOffset.UtcNow);
                json.WriteString("severity", Severity(logEntry.LogLevel));
                json.WriteString("message", message);
                json.WriteString("logger", logEntry.Category);
                if (logEntry.Exception is not null)
                {
                    json.WriteString("exception", logEntry.Exception.ToString());
                }
                json.WriteEndObject();
            }
            textWriter.WriteLine(Encoding.UTF8.GetString(buffer.ToArray()));
        }

        private static string Severity(LogLevel level) => level switch
        {
            LogLevel.Trace => "TRACE",
            LogLevel.Debug => "DEBUG",
            LogLevel.Information => "INFO",
            LogLevel.Warning => "WARN",
            LogLevel.Error => "ERROR",
            _ => "FATAL",
        };
    }
}
