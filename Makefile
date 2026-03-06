GOBIN ?= $(shell go env GOPATH)/bin
BINARY = openbotkit
ALIAS = obk

.PHONY: build install uninstall

build:
	go build -o $(BINARY) .

install:
	go install .
	ln -sf $(GOBIN)/$(BINARY) $(GOBIN)/$(ALIAS)

uninstall:
	rm -f $(GOBIN)/$(BINARY) $(GOBIN)/$(ALIAS)
