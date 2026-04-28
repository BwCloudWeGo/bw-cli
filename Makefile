GO ?= go
PROTOC ?= protoc
PROTO_PATH := api/proto
PROTO_OUT := api/gen
PROTO_PLUGIN_PATH := $(shell go env GOPATH)/bin
PROTO_FILES := $(shell if [ -d "$(PROTO_PATH)" ]; then cd $(PROTO_PATH) && find . -name '*.proto' | sed 's,^\./,,'; fi)

.PHONY: proto test tidy run-gateway run-user run-note run-cli install-cli install-bw-cli tools

tools:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	@if [ -z "$(PROTO_FILES)" ]; then \
		echo "No proto files found"; \
	else \
		PATH="$(PROTO_PLUGIN_PATH):$$PATH" $(PROTOC) \
			--proto_path=$(PROTO_PATH) \
			--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
			--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
			$(PROTO_FILES); \
	fi

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

run-user:
	$(GO) run ./cmd/user

run-note:
	$(GO) run ./cmd/note

run-gateway:
	$(GO) run ./cmd/gateway

run-cli:
	$(GO) run ./cmd/bw-cli

install-cli:
	$(GO) install ./cmd/bw-cli

install-bw-cli: install-cli
