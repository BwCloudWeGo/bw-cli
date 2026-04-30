package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

func TestAddServiceWritesCompleteServiceStructure(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "order-item",
		Port:     9103,
		RunProto: false,
	})

	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, "cmd", "order_item", "main.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "model", "order_item.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "model", "repository.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "service", "service.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "service", "service_test.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "repo", "gorm_repository.go"))
	require.FileExists(t, filepath.Join(root, "internal", "order_item", "handler", "server.go"))
	require.FileExists(t, filepath.Join(root, "api", "proto", "order_item", "v1", "order_item.proto"))
	require.FileExists(t, filepath.Join(root, "docs", "services", "order_item.md"))

	proto := readString(t, filepath.Join(root, "api", "proto", "order_item", "v1", "order_item.proto"))
	require.Contains(t, proto, "package order_item.v1;")
	require.Contains(t, proto, "option go_package = \"github.com/acme/app/api/gen/order_item/v1;orderitemv1\";")
	require.Contains(t, proto, "service OrderItemService")

	mainFile := readString(t, filepath.Join(root, "cmd", "order_item", "main.go"))
	require.Contains(t, mainFile, "const serviceName = \"order-item-service\"")
	require.Contains(t, mainFile, "const defaultGRPCPort = 9103")
	require.Contains(t, mainFile, "const grpcPortEnv = \"APP_ORDER_ITEM_GRPC_PORT\"")
	require.Contains(t, mainFile, "orderitemv1.RegisterOrderItemServiceServer")
	require.Contains(t, mainFile, "internal/order_item/service")

	makefile := readString(t, filepath.Join(root, "Makefile"))
	require.Contains(t, makefile, ".PHONY: proto test run-order_item")
	require.Contains(t, makefile, "run-order_item:")
	require.Contains(t, makefile, "$(GO) run ./cmd/order_item")

	doc := readString(t, filepath.Join(root, "docs", "services", "order_item.md"))
	require.Contains(t, doc, "bw-cli service order-item --port 9103")
	require.Contains(t, doc, "APP_ORDER_ITEM_GRPC_PORT")
}

func TestAddServiceRejectsExistingService(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "order"), 0o755))

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "order",
		RunProto: false,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestAddServiceRejectsInvalidName(t *testing.T) {
	root := t.TempDir()
	writeServiceProject(t, root)

	err := scaffold.AddService(scaffold.ServiceOptions{
		RootDir:  root,
		Name:     "../bad",
		RunProto: false,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "service name")
}

func writeServiceProject(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod": "module github.com/acme/app\n\ngo 1.25.0\n",
		"Makefile": `GO ?= go

.PHONY: proto test

proto:
	$(GO) run ./tools/protogen

test:
	$(GO) test ./...
`,
		"tools/protogen/main.go": "package main\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}
