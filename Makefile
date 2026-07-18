GO ?= go
BINARY ?= bin/qwen-observe

.PHONY: build check test validate snapshot clean

build:
	@mkdir -p bin
	$(GO) build -o $(BINARY) ./cmd/qwen-observe

check: test
	$(GO) vet ./...
	git diff --check

test:
	$(GO) test ./...

validate: build
	./$(BINARY) validate

snapshot: build
	./$(BINARY) snapshot --output evidence/live-snapshot.json
	./$(BINARY) report --input evidence/live-snapshot.json --output evidence/live-report.md

clean:
	rm -f $(BINARY)
