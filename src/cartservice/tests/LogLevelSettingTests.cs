using System;
using cartservice.logging;
using Microsoft.Extensions.Logging;
using Xunit;

namespace cartservice.tests;

public class LogLevelSettingTests
{
    [Theory]
    [InlineData(null, LogLevel.Information)]
    [InlineData("", LogLevel.Information)]
    [InlineData("info", LogLevel.Information)]
    [InlineData("Info", LogLevel.Information)]
    [InlineData("debug", LogLevel.Debug)]
    [InlineData("DEBUG", LogLevel.Debug)]
    [InlineData("warn", LogLevel.Warning)]
    [InlineData("error", LogLevel.Error)]
    public void Parse_AcceptsTheSameValuesAsTheOtherServices(string value, LogLevel expected)
    {
        Assert.Equal(expected, LogLevelSetting.Parse(value));
    }

    [Theory]
    [InlineData("verbose")]
    [InlineData("trace")]
    [InlineData("critical")]
    [InlineData("none")]
    [InlineData(" info")]
    public void Parse_RejectsAnythingElse(string value)
    {
        // .NET's own names (trace, critical, none) are refused too: the other services refuse them.
        Assert.Throws<ArgumentException>(() => LogLevelSetting.Parse(value));
    }
}
