BINARY = qc
CMD_DIR = ./cmd/cli
DIST_DIR = dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -s -w -X main.appVersion=$(VERSION)

PLATFORMS = darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

.PHONY: build build-all release test lint coverage race clean run

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD_DIR)

build-all: clean-dist
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(DIST_DIR)/$(BINARY)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then output="$$output.exe"; fi; \
		echo "Building $$os/$$arch → $$output"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $$output $(CMD_DIR); \
	done

release: build-all
	@echo "---"
	@echo "Release binaries: $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)/
	@echo "---"
	cd $(DIST_DIR) && shasum -a 256 * > checksums.txt
	@echo "Checksums: $(DIST_DIR)/checksums.txt"

test:
	go test ./... -count=1

lint:
	go vet ./...

coverage:
	go test ./... -count=1 -cover

race:
	go test ./... -count=1 -race

clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)
	go clean ./...

clean-dist:
	rm -rf $(DIST_DIR)

run: build
	./$(BINARY) $(ARGS)
