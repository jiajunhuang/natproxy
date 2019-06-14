default: fmt vet
	protoc -I pb/ natproxy.proto --go_out=plugins=grpc:pb/
	go build -o bin/natproxy cmd/natproxy/main.go
	go build -o bin/server cmd/server/main.go
	grep version client/client.go | head -n 1

fmt:
	go fmt ./...

vet:
	go vet ./...
