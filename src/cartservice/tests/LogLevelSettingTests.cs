using System;
using cartservice.logging;
using Microsoft.Extensions.Logging;
using Xunit;

namespace cartservice.tests
{
    public class LogLevelSettingTests
    {
        [Theory]
        [InlineData(null, LogLevel.Information)]
        [InlineData("", LogLevel.Information)]
        [InlineData("info", LogLevel.Information)]
        [InlineData("debug", LogLevel.Debug)]
        [InlineData("DEBUG", LogLevel.Debug)]
        [InlineData("warn", LogLevel.Warning)]
        [InlineData("error", LogLevel.Error)]
        public void Parse_AcceptsTheSameValuesAsTheOtherServices(string value, LogLevel expected)
        {
            Assert.Equal(expected, LogLevelSetting.Parse(value));
        }

        [Fact]
        public void Parse_RejectsAnUnknownLevel()
        {
            Assert.Throws<ArgumentException>(() => LogLevelSetting.Parse("verbose"));
        }
    }
}
