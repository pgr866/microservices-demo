using System;
using System.Buffers;
using System.IO;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Microsoft.Extensions.Logging.Console;

namespace cartservice.logging;

/// <summary>One JSON line per entry, with the same field names as the other services' logs.</summary>
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

        textWriter.WriteLine(FormatLine(logEntry.LogLevel, logEntry.Category, message, logEntry.Exception));
    }

    /// <summary>Also usable before the logging host exists (an invalid LOG_LEVEL).</summary>
    public static string FormatLine(LogLevel level, string category, string message, Exception exception = null)
    {
        // Utf8JsonWriter, not JsonSerializer: reflection-based serialization breaks in the trimmed image.
        var buffer = new ArrayBufferWriter<byte>();
        // Relaxed escaping keeps ' or + readable; the strict default only matters for JSON inside HTML.
        using (var json = new Utf8JsonWriter(buffer, new JsonWriterOptions { Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping }))
        {
            json.WriteStartObject();
            json.WriteString("timestamp", DateTimeOffset.UtcNow);
            json.WriteString("severity", Severity(level));
            json.WriteString("message", message);
            json.WriteString("logger", category);
            if (exception is not null)
            {
                json.WriteString("exception", exception.ToString());
            }
            json.WriteEndObject();
        }
        return Encoding.UTF8.GetString(buffer.WrittenSpan);
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
