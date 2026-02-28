.PHONY: build build-windows run test lint clean

BINARY=satvos-connector

build:
	go build -o bin/$(BINARY) ./cmd/connector

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY).exe ./cmd/connector

run:
	go run ./cmd/connector

test:
	go test ./... -v -count=1 -race

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
