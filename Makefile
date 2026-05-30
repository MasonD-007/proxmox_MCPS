.PHONY: build test clean

build:
	go build ./...

test:
	go test ./...

clean:
	go clean
	rm -f proxmox-mcp
