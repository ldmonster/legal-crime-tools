BINARY  := legal-crime-tools
BINDIR  := ./bin
CMD     := ./cmd/legal-crime-tools

.PHONY: build clean

build:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/$(BINARY) $(CMD)

clean:
	rm -rf $(BINDIR)
