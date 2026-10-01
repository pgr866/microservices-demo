using System;
using System.IO;
using System.Text.Json;
using cartservice.logging;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace cartservice.tests
{
    public class JsonLogFormatterTests
    {
        private static JsonElement Format(LogLevel level, string message, Exception exception = null)
        {
            var output = new StringWriter();
            var entry = new LogEntry<string>(level, "cartservice.Test", new EventId(0), message, exception, (state, _) => state);

            new JsonLogFormatter().Write(entry, null, output);

            return JsonDocument.Parse(output.ToString()).RootElement;
        }

        [Fact]
        public void Write_UsesTheSameFieldsAsTheOtherServices()
        {
            var line = Format(LogLevel.Information, "GetCart");

            Assert.Equal("INFO", line.GetProperty("severity").GetString());
            Assert.Equal("GetCart", line.GetProperty("message").GetString());
            Assert.Equal("cartservice.Test", line.GetProperty("logger").GetString());
            Assert.True(DateTimeOffset.TryParse(line.GetProperty("timestamp").GetString(), out _));
            Assert.False(line.TryGetProperty("exception", out _));
        }

        [Fact]
        public void Write_KeepsTheMessageReadable()
        {
            var output = new StringWriter();
            var entry = new LogEntry<string>(LogLevel.Information, "cartservice.Test", new EventId(0), "Can't stop: press Ctrl+C", null, (state, _) => state);

            new JsonLogFormatter().Write(entry, null, output);

            Assert.Contains("\"message\":\"Can't stop: press Ctrl+C\"", output.ToString());
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
        [InlineData(LogLevel.Warning, "WARN")]
        [InlineData(LogLevel.Critical, "FATAL")]
        public void Write_NamesEverySeverity(LogLevel level, string severity)
        {
            Assert.Equal(severity, Format(level, "message").GetProperty("severity").GetString());
        }

        [Fact]
        public void Write_SkipsEmptyEntries()
        {
            var output = new StringWriter();
            var entry = new LogEntry<string>(LogLevel.Information, "cartservice.Test", new EventId(0), null, null, (_, _) => null);

            new JsonLogFormatter().Write(entry, null, output);

            Assert.Equal("", output.ToString());
        }
    }
}
