.PHONY: build run test clean sample lint

BINARY := kgraph
PKG    := ./cmd/kgraph

build:
	go build -o $(BINARY) $(PKG)

run: build
	./$(BINARY) --input data/industrial-revolution.txt --output knowledge_graph.html

sample: build
	./$(BINARY) --test --output sample_graph.html

debug: build
	./$(BINARY) --input data/industrial-revolution.txt --output knowledge_graph.html --debug

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) *.html *.json
