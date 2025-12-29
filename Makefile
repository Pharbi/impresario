.PHONY: build build-all clean install

BINARY_NAME=impresario
VERSION=1.0.0

build:
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME) .

build-all:
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-windows-amd64.exe .

install: build
	cp bin/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)

clean:
	rm -rf bin/
