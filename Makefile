.PHONY: build run clean

BINARY=whatskel

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY) -config config.toml

clean:
	rm -f $(BINARY)
	rm -f whatsapp-session.db whatsapp-store.db
