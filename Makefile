BIN := bin/pulumi-resource-coolify
VERSION ?= 0.1.0

.PHONY: build test vet schema gen-sdk clean

build:
	go build -o $(BIN) ./cmd/pulumi-resource-coolify

test:
	go test ./...

vet:
	go vet ./...

schema: build
	pulumi package get-schema $(BIN) > schema.json

gen-sdk: build
	pulumi package gen-sdk $(BIN)

clean:
	rm -rf bin sdk