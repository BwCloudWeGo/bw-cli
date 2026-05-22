package scaffold_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/scaffold"
)

func TestInitWithoutDemoRemovesBusinessResidue(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	writeSourceWithBusinessResidue(t, source)

	err := scaffold.Init(scaffold.InitOptions{
		SourceDir:   source,
		TargetDir:   target,
		ModulePath:  "github.com/acme/clean",
		IncludeDemo: false,
	})

	require.NoError(t, err)
	requireNoPath(t, filepath.Join(target, "cmd", "order"))
	requireNoPath(t, filepath.Join(target, "internal", "order"))
	requireNoPath(t, filepath.Join(target, "api", "proto", "order"))
	requireNoPath(t, filepath.Join(target, "api", "gen", "order"))
	requireNoPath(t, filepath.Join(target, "internal", "gateway", "client"))
	requireNoPath(t, filepath.Join(target, "internal", "gateway", "handler"))
	requireNoPath(t, filepath.Join(target, "internal", "gateway", "request"))
	requireNoPath(t, filepath.Join(target, "internal", "gateway", "router", "order_routes.go"))
	requireNoPath(t, filepath.Join(target, "docs", "services"))

	require.FileExists(t, filepath.Join(target, "cmd", "gateway", "main.go"))
	require.FileExists(t, filepath.Join(target, "internal", "gateway", "router", "health.go"))
	require.FileExists(t, filepath.Join(target, "internal", "gateway", "router", "router.go"))
	require.FileExists(t, filepath.Join(target, "internal", "gateway", "router", "v1.go"))
}

func writeSourceWithBusinessResidue(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"go.mod":                                    "module old/module\n\ngo 1.25.0\n",
		"README.md":                                 "old/module scaffold\n",
		"Makefile":                                  "run-order:\n\tgo run ./cmd/order\n",
		"configs/config.yaml":                       "app:\n  name: demo\n",
		"cmd/bw-cli/main.go":                        "package main\n",
		"cmd/gateway/main.go":                       "package main\n",
		"cmd/order/main.go":                         "package main\n",
		"internal/order/model/order.go":             "package model\n",
		"api/proto/order/v1/order.proto":            "option go_package = \"old/module/api/gen/order/v1;orderv1\";\n",
		"api/gen/order/v1/order.pb.go":              "package orderv1\n",
		"internal/gateway/client/clients.go":        "package client\n",
		"internal/gateway/handler/order_handler.go": "package handler\n",
		"internal/gateway/request/order_request.go": "package request\n",
		"internal/gateway/router/health.go":         "package router\n",
		"internal/gateway/router/router.go":         "package router\n",
		"internal/gateway/router/v1.go":             "package router\n",
		"internal/gateway/router/order_routes.go":   "package router\n\nimport _ \"old/module/internal/gateway/handler\"\n",
		"internal/gateway/router/router_test.go":    "package router\n",
		"pkg/scaffold/scaffold.go":                  "package scaffold\n",
		"pkg/alipayx/alipayx.go":                    "package alipayx\n",
		"pkg/kafkax/kafkax.go":                      "package kafkax\n",
		"pkg/esx/esx.go":                            "package esx\n",
		"pkg/middleware/jwt.go":                     "package middleware\n",
		"tools/protogen/main.go":                    "package main\n",
		"docs/services/order.md":                    "old order service doc\n",
		"docs/superpowers/specs/old.md":             "old spec\n",
		"docs/toolkit.md":                           "old toolkit\n",
		"docs/mongodb.md":                           "old mongodb\n",
		"docs/alipay.md":                            "old alipay\n",
		"docs/elasticsearch.md":                     "old elasticsearch\n",
		"docs/mongo-call-examples.md":               "old mongo examples\n",
		"docs/architecture.md":                      "old architecture\n",
		"docs/usage.md":                             "old usage\n",
		"docker-compose.yml":                        "services: {}\n",
		"Dockerfile":                                "FROM scratch\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
}

func requireNoPath(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "expected %s to be removed", path)
}
