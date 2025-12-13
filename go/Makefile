.PHONY: build run run-stdout clean

build:
	go build -o bin/feedparser ./cmd/feedparser

run: build
	./bin/feedparser --config=../config.json

run-stdout: build
	./bin/feedparser --config=../config.json --stdout

clean:
	rm -rf bin feedparser.log
