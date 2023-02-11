module github.com/go-msvc/nats-utils

go 1.18

replace github.com/go-msvc/utils => ../utils

replace github.com/go-msvc/utils/ms => ../utils/ms

require (
	github.com/go-msvc/config v0.0.2
	github.com/go-msvc/errors v1.2.0
	github.com/go-msvc/logger v1.0.0
	github.com/go-msvc/utils v0.0.0-20230117192331-a72280d31e5a
	github.com/google/uuid v1.3.0
	github.com/nats-io/nats.go v1.23.0
)

require (
	github.com/go-msvc/data v1.0.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/nats-io/nats-server/v2 v2.9.14 // indirect
	github.com/nats-io/nkeys v0.3.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)
