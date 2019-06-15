build: fmt vet
	./package.sh

fmt:
	go fmt ./...

vet:
	go vet ./...
