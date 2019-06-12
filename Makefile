default: fmt
	protoc -I pb/ natproxy.proto --go_out=plugins=grpc:pb/
	go build -o bin/natproxy cmd/natproxy/main.go
	go build -o bin/server cmd/server/main.go

fmt:
	go fmt ./...
