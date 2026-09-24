# Cart Service

The Cart service stores the items in a user's shopping cart in Redis (falling back to an in-memory store if `REDIS_ADDR` isn't set) and exposes it via gRPC.

## Testing

```bash
dotnet test tests/cartservice.tests.csproj

# Coverage report (Cobertura XML, under tests/TestResults/) — excludes generated
# protobuf/gRPC code (tests/coverlet.runsettings), which would otherwise dilute the number
dotnet test tests/cartservice.tests.csproj --collect:"XPlat Code Coverage" --settings tests/coverlet.runsettings
```

## Linting

```bash
dotnet format --verify-no-changes
```
