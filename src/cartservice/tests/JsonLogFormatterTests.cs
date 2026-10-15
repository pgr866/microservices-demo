using System;
using System.IO;
using System.Text.Json;
using cartservice.logging;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace cartservice.tests;

public class JsonLogFormatterTests
{
    private static string Write(LogLevel level, string message, Exception exception = null)
    {
        var output = new StringWriter();
        var entry = new LogEntry<string>(level, "cartservice.Test", new EventId(0), message, exception, (state, _) => state);
        new JsonLogFormatter().Write(entry, null, output);
        return output.ToString();
    }

    private static JsonElement Format(LogLevel level, string message, Exception exception = null) =>
        JsonDocument.Parse(Write(level, message, exception)).RootElement;

    [Fact]
    public void Write_UsesTheSameFieldsAsTheOtherServices()
    {
        var line = Format(LogLevel.Information, "GetCart");

        Assert.Equal("INFO", line.GetProperty("severity").GetString());
        Assert.Equal("GetCart", line.GetProperty("message").GetString());
        Assert.Equal("cartservice.Test", line.GetProperty("logger").GetString());
        Assert.True(line.GetProperty("timestamp").TryGetDateTimeOffset(out var timestamp));
        Assert.Equal(TimeSpan.Zero, timestamp.Offset);
        Assert.False(line.TryGetProperty("exception", out _));
    }

    [Fact]
    public void FormatLine_WritesAnInvalidLogLevelAsOneJsonLine()
    {
        var message = Assert.Throws<ArgumentException>(() => LogLevelSetting.Parse("verbose")).Message;

        var text = JsonLogFormatter.FormatLine(LogLevel.Error, "cartservice", message);

        Assert.DoesNotContain("\n", text);
        var line = JsonDocument.Parse(text).RootElement;
        Assert.Equal("ERROR", line.GetProperty("severity").GetString());
        Assert.Equal("cartservice", line.GetProperty("logger").GetString());
        Assert.Contains("invalid LOG_LEVEL \"verbose\"", line.GetProperty("message").GetString());
        Assert.True(line.GetProperty("timestamp").TryGetDateTimeOffset(out _));
    }

    [Fact]
    public void Write_OneLinePerEntry()
    {
        var output = Write(LogLevel.Information, "first\nsecond");

        Assert.Equal(Environment.NewLine, output[^Environment.NewLine.Length..]);
        Assert.DoesNotContain('\n', output.TrimEnd());
    }

    [Fact]
    public void Write_KeepsTheMessageReadable()
    {
        Assert.Contains("\"message\":\"Can't stop: press Ctrl+C\"", Write(LogLevel.Information, "Can't stop: press Ctrl+C"));
    }

    [Fact]
    public void Write_IncludesTheException()
    {
        var line = Format(LogLevel.Error, "Can't access cart storage", new InvalidOperationException("storage down"));

        Assert.Equal("ERROR", line.GetProperty("severity").GetString());
        Assert.Contains("storage down", line.GetProperty("exception").GetString());
    }

    [Theory]
    [InlineData(LogLevel.Trace, "TRACE")]
    [InlineData(LogLevel.Debug, "DEBUG")]
    [InlineData(LogLevel.Information, "INFO")]
    [InlineData(LogLevel.Warning, "WARN")]
    [InlineData(LogLevel.Error, "ERROR")]
    [InlineData(LogLevel.Critical, "FATAL")]
    public void Write_NamesEverySeverity(LogLevel level, string severity)
    {
        Assert.Equal(severity, Format(level, "message").GetProperty("severity").GetString());
    }

    [Fact]
    public void Write_KeepsAnExceptionLoggedWithoutAMessage()
    {
        var output = new StringWriter();
        var entry = new LogEntry<string>(LogLevel.Error, "cartservice.Test", new EventId(0), null, new InvalidOperationException("storage down"), null);

        new JsonLogFormatter().Write(entry, null, output);

        var line = JsonDocument.Parse(output.ToString()).RootElement;
        Assert.Equal(JsonValueKind.Null, line.GetProperty("message").ValueKind);
        Assert.Contains("storage down", line.GetProperty("exception").GetString());
    }

    [Fact]
    public void Write_SkipsEmptyEntries()
    {
        Assert.Equal("", Write(LogLevel.Information, null));
    }
}
