module github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice

go 1.25.0

toolchain go1.27.1

require (
	github.com/google/uuid v1.6.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.10.2
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.46.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260825221802-da73d73af1c5 // indirect
)
