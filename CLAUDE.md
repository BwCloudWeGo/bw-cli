# Project Instructions

## Tech Stack

- Go 1.25 module: `github.com/BwCloudWeGo/bw-cli`.
- HTTP Gateway uses Gin under `internal/gateway`.
- Internal services use gRPC and protobuf from `api/proto`, generated into `api/gen`.
- Persistence uses Gorm for relational databases and MongoDB driver via `pkg/mongox`.
- Config loads from `configs/config.yaml` or optional Nacos via `configs/nacos.yaml`.
- Logging uses Zap plus Lumberjack through `pkg/logger`.

## Build And Run

- Install proto tools: `make tools`
- Generate protobuf code: `make proto`
- Run all tests: `make test`
- Tidy modules: `make tidy`
- Run services: `make run-user`, `make run-note`, `make run-order`, `make run-gateway`
- Run CLI locally: `make run-cli`
- Install CLI locally: `make install-cli`

## Project Structure

- `cmd/gateway`: HTTP gateway process.
- `cmd/user`, `cmd/note`, `cmd/order`: demo gRPC services.
- `cmd/bw-cli`: scaffold CLI entrypoint.
- `api/proto`: protobuf source contracts.
- `api/gen`: generated protobuf Go code.
- `internal/gateway`: routes, HTTP request DTOs, handlers, and gRPC clients.
- `internal/<service>`: DDD-style service implementation.
- `pkg/*`: reusable infrastructure packages.
- `tools/protogen`: cross-platform protobuf generation helper.
- `docs/onboarding.md`: project architecture, API, request flow, and dev guide.

## Service Layering

- `entity`: business aggregate, sentinel errors, and repository interface.
- `dto`: service command inputs and output DTO conversion.
- `service`: use-case orchestration, no Gin/gRPC/database objects.
- `model`: Gorm and MongoDB persistence structs only.
- `repo`: persistence implementation and entity/model mapping.
- `handler`: gRPC request/response adaptation and error mapping.
- Gateway handlers only bind HTTP input, call gRPC clients, and return `pkg/httpx` responses.

## Conventions

- Test files use `*_test.go`.
- Proto files use `api/proto/<service>/v1/<service>.proto`.
- Generated packages use names such as `userv1`, `notev1`, and `orderv1`.
- Add business errors in `entity`, map them in gRPC `handler`, and convert gRPC errors back in Gateway with `pkg/errors.FromGRPC`.
- Keep config-driven service names, ports, and targets in `configs/config.yaml` or Nacos.
- Do not edit generated files in `api/gen` by hand; edit proto and run `make proto`.

## Adding A Service

1. Prefer `bw-cli service <name> --port <port> --tidy` for a full skeleton.
2. Update `api/proto/<name>/v1/*.proto`, then run `make proto`.
3. Implement `entity`, `dto`, `service`, `model`, `repo`, and gRPC `handler`.
4. Add Gateway `request`, `handler`, and `router` entries when the service needs HTTP APIs.
5. Add focused tests and run `make test`.

## Testing Notes

- Run `go test ./...` or `make test` before claiming completion.
- For Gateway route changes, cover `internal/gateway/router` and `internal/gateway/request`.
- For business behavior, test the service layer first.
- For scaffold changes, cover generation and deletion paths because they rewrite many files.

## Git Style

- Recent commits mostly follow Conventional Commits, for example `feat(scaffold): ...`.
- Prefer `<type>(scope): 中文动词开头摘要` with `feat`, `fix`, `refactor`, `docs`, `test`, or `chore`.
