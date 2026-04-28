BINARY := esi-lsp
MODULE := github.com/vikranthBala/esi-lsp

.PHONY: all build clean test gen extension help

all: build extension

## build: Build the language server binary
build:
	go build -o $(BINARY) ./cmd/esi-lsp

## build-all: Build for all platforms
build-all:
	GOOS=linux   GOARCH=amd64 go build -o dist/$(BINARY)-linux-amd64   ./cmd/esi-lsp
	GOOS=darwin  GOARCH=amd64 go build -o dist/$(BINARY)-darwin-amd64  ./cmd/esi-lsp
	GOOS=darwin  GOARCH=arm64 go build -o dist/$(BINARY)-darwin-arm64  ./cmd/esi-lsp
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY)-windows-amd64.exe ./cmd/esi-lsp

## test: Run all tests
test:
	go test ./...

## gen: Regenerate tag rules from tags.json (requires Ollama running)
gen:
	go run tools/gen_tags/main.go > internal/analyzer/rules_gen.go

## extension: Compile the VS Code extension
extension:
	cd vscode-extension && npm run compile

## clean: Remove build artifacts
clean:
	rm -f $(BINARY)
	rm -rf dist/
	rm -rf vscode-extension/out/

## tidy: Tidy go modules
tidy:
	go mod tidy

## help: Show this help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
