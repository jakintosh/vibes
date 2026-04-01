BINARY  := transit
PORT    := 8080
URL     ?=

TESTDATA_PREV := testdata/previous.zip
TESTDATA_CURR := testdata/current.zip

.PHONY: build test run clean testdata

build:
	go build -o $(BINARY) .

test:
	go test ./...

run: build
ifndef URL
	$(error URL is required — usage: make run URL=https://example.com/gtfs.zip)
endif
	./$(BINARY) --url $(URL) --port $(PORT)

render: build
ifndef URL
	$(error URL is required — usage: make render URL=https://example.com/gtfs.zip OUT=out.html)
endif
ifndef OUT
	$(error OUT is required — usage: make render URL=https://example.com/gtfs.zip OUT=out.html)
endif
	./$(BINARY) --url $(URL) --render $(OUT)

testdata: $(TESTDATA_PREV) $(TESTDATA_CURR)

$(TESTDATA_PREV) $(TESTDATA_CURR): testdata/gen_feeds.py
	python3 testdata/gen_feeds.py

clean:
	rm -f $(BINARY)
	rm -f $(TESTDATA_PREV) $(TESTDATA_CURR)
