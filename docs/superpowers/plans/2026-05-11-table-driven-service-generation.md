# Table-Driven Service Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `bw-cli service` so it can generate service code from existing relational database tables and optional relation schema files while preserving the current default template behavior.

**Architecture:** Keep `pkg/scaffold/service.go` as the orchestration entry point, but move table-specific concerns into focused helpers: schema parsing, relational database introspection, table metadata validation, and field-aware template data. Existing fixed-field generation stays the fallback path. Table-driven mode builds richer template data before any file is written.

**Tech Stack:** Go, standard `database/sql`, Gorm drivers already in the project, Viper config loader, `gopkg.in/yaml.v3`, existing `pkg/scaffold` templates and tests.

---

## File Structure

- Modify `cmd/bw-cli/main.go`: parse `--table`, `--schema`, and `--yes`; pass them into `scaffold.ServiceOptions`.
- Modify `cmd/bw-cli/main_test.go`: cover new option parsing and legacy behavior.
- Modify `pkg/scaffold/service.go`: extend `ServiceOptions`, branch into table-driven metadata loading, and render fields dynamically.
- Create `pkg/scaffold/service_schema.go`: YAML schema structs, defaults, and validation helpers.
- Create `pkg/scaffold/service_schema_test.go`: schema parsing and relation validation tests.
- Create `pkg/scaffold/introspection.go`: relational metadata types and SQLite/MySQL/PostgreSQL introspection.
- Create `pkg/scaffold/introspection_test.go`: SQLite-backed table, primary key, field type, and relation validation tests.
- Modify `pkg/scaffold/service_test.go`: add table-driven generation tests while keeping existing default-template tests.
- Modify `docs/usage.md`, `README.md`, and service documentation template in `pkg/scaffold/service.go`: document `--table`, `--schema`, relation YAML, and limits.

## Task 1: CLI Options and ServiceOptions

- [ ] Add fields to `scaffold.ServiceOptions`: `Table`, `SchemaPath`, and `AssumeYes`.
- [ ] Update `parseServiceOptions` to parse `--table`, `--schema`, and `--yes`.
- [ ] Add tests in `cmd/bw-cli/main_test.go` for legacy parsing and new flags.
- [ ] Run `go test ./cmd/bw-cli`.

## Task 2: Schema Parsing

- [ ] Create `pkg/scaffold/service_schema.go` with `ServiceSchema`, `RelationSchema`, `loadServiceSchema`, `mergeServiceSchema`, and relation type validation.
- [ ] Support `table`, `resource`, `exclude_fields`, `readonly_fields`, and `relations`.
- [ ] Add tests for valid YAML, command table override, unsupported relation type, and missing relation fields.
- [ ] Run `go test ./pkg/scaffold -run 'Schema|ServiceSchema'`.

## Task 3: Relational Introspection

- [ ] Create `pkg/scaffold/introspection.go` with `TableMetadata`, `ColumnMetadata`, `RelationMetadata`, `inspectRelationalSchema`, and field type mapping helpers.
- [ ] Support SQLite via `PRAGMA table_info`, MySQL via `information_schema.columns`, and PostgreSQL via `information_schema`.
- [ ] Validate primary table existence, associated table existence, primary key count, and relation fields.
- [ ] Add SQLite tests for table existence, missing table, primary key detection, mapped field types, and missing relation fields.
- [ ] Run `go test ./pkg/scaffold -run 'Inspect|Introspection|Relation'`.

## Task 4: Dynamic Template Data

- [ ] Extend `serviceTemplateData` with field lists, primary key metadata, relation metadata, and pre-rendered code/proto fragments.
- [ ] Preserve existing fixed `Name`/`Description` metadata when table-driven mode is inactive.
- [ ] Convert model, DTO, command, proto, Gorm repo, handler, and gateway templates to use field-aware fragments.
- [ ] Keep Mongo repository generation on the legacy primary resource shape for now, documented as unchanged.
- [ ] Run existing `go test ./pkg/scaffold -run TestAddServiceWritesCompleteServiceStructure`.

## Task 5: Relation Query Generation

- [ ] Generate associated table model and DTO files for relation tables.
- [ ] Add `model.QueryRepository` and Gorm query methods for `has_many`, `has_one`, and `belongs_to`.
- [ ] Add service, handler, proto, and gateway methods for configured or derived relation method names.
- [ ] Add a table-driven multi-table generation test that asserts relation files and methods exist.
- [ ] Run `go test ./pkg/scaffold -run 'TableDriven|Relation|AddService'`.

## Task 6: Docs and Full Verification

- [ ] Update `README.md` and `docs/usage.md` with table-driven examples, YAML schema, error behavior, and limitations.
- [ ] Update generated service docs template with a table-driven section.
- [ ] Run `gofmt` on modified Go files.
- [ ] Run `go test ./...`.
- [ ] Review `git diff` for unrelated changes.
- [ ] Commit all implementation and docs changes.
- [ ] Push `codex/table-driven-service-generation` to `origin`.
