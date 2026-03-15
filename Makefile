.PHONY: build test clean install lint

BINARY=tracescope
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/tracescope

test:
	go test ./... -race -v

clean:
	rm -rf $(BUILD_DIR) .tracescope

install:
	go install ./cmd/tracescope

lint:
	golangci-lint run ./...
