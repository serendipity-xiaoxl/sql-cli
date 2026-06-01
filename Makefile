BINARY = qc
CMD_DIR = ./cmd/cli

.PHONY: build test lint coverage race clean run

build:
	go build -o $(BINARY) $(CMD_DIR)

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
	go clean ./...

run: build
	./$(BINARY) $(ARGS)
