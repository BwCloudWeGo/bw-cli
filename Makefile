GO ?= go
PROTOC ?= protoc
PROTO_PATH ?= api/proto
PROTO_OUT ?= api/gen

export PROTOC
export PROTO_PATH
export PROTO_OUT

.PHONY: proto test tidy run-gateway run-user run-note run-cli install-cli install-bw-cli tools

tools:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	$(GO) run ./tools/protogen

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
