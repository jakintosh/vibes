BINARY  := transit
PORT    := 8080
URL     ?=

.PHONY: build test run clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

run: build
ifndef URL
	$(error URL is required — usage: make run URL=https://example.com/gtfs.zip)
endif
	./$(BINARY) --url $(URL) --port $(PORT)

clean:
	rm -f $(BINARY)
