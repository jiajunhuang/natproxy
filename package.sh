#!/bin/bash

go fmt ./...
go vet ./...
protoc -I pb/ natproxy.proto --go_out=plugins=grpc:pb/
version=`grep version client/client.go | head -n 1 | tr " " "\n" | tail -n 1 | tr -d '"'`
go build -o bin/natproxys cmd/server/main.go

echo "version is ${version}"

os_all='linux windows darwin'
arch_all='amd64'

GOOS=linux GOARCH=arm64 go build -o bin/natproxy-linux-arm64-${version} cmd/natproxy/main.go

for os in $os_all; do
    for arch in $arch_all; do
        echo "building natproxy client for natproxy-${os}-${arch}-${version}"
        if [ "x${os}" = x"windows" ]; then
            GOOS=${os} GOARCH=${arch} go build -o bin/natproxy-${os}-${arch}-${version}.exe cmd/natproxy/main.go
        else
            GOOS=${os} GOARCH=${arch} go build -o bin/natproxy-${os}-${arch}-${version} cmd/natproxy/main.go
        fi
    done
done
