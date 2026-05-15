# Table-Driven Service Generation Design

Date: 2026-05-11

## Context

`bw-cli service <name>` currently generates a complete CRUD service from fixed placeholder fields (`Name` and `Description`). It does not let users bind the generated service to an existing database table. As a result, users still need to manually rewrite model, DTO, proto, repository, handler, gateway request, and documentation files after generation.

This design extends the existing `service` command so a user can generate service code from configured relational database tables. The first version only supports relational databases already represented by the project configuration: SQLite, MySQL, and PostgreSQL. MongoDB generation remains unchanged.

## Goals

- Preserve the current `bw-cli service <name>` behavior when no table options are provided.
- Let users generate a service from a primary table with `--table`.
- Let users describe multi-table relationships with a YAML schema file.
- Validate configured database connectivity, table existence, primary keys, and relation fields before writing files.
- Generate code fields from table metadata instead of placeholder fields.
- Support relation query methods for associated tables while keeping the primary table as the public service resource.

## Non-Goals

- Do not implement MongoDB schema introspection in the first version.
- Do not generate separate public CRUD APIs for associated tables.
- Do not support composite primary keys or composite relation keys in the first version.
- Do not generate optimized SQL joins by default.
- Do not silently fall back to placeholder templates in non-interactive environments.

## Recommended Approach

Use a gradual extension of the existing `bw-cli service` command.

The existing command remains valid:

```bash
bw-cli service order
```

This continues to generate the current placeholder CRUD service.

Table-driven generation is enabled by either `--table` or `--schema`:

```bash
bw-cli service order --table orders
bw-cli service order --table orders --schema configs/services/order.yaml
```

This keeps the user experience simple for single-table services while allowing structured configuration for multi-table services.

## Command Contract

Add these options to `bw-cli service`:

```text
--table string    Primary relational table used for table-driven generation.
--schema string   YAML file describing fields, exclusions, readonly fields, and relations.
--yes             In a TTY only, automatically confirm fallback to the default template when database configuration is unavailable.
```

Rules:

- If neither `--table` nor `--schema` is provided, generation behavior is unchanged.
- If `--table` or `--schema` is provided, the command enters table-driven mode.
- If both command-line `--table` and YAML `table` are provided, `--table` wins.
- If table-driven mode cannot read a usable relational database configuration:
  - In a TTY, ask whether to continue with the default placeholder template.
  - In a non-TTY, fail before writing files and tell the user to configure the relational database first.
  - `--yes` only answers the TTY fallback prompt. It does not allow non-TTY table-driven generation to fall back silently.
- If table-driven mode reaches database validation and any table/field/primary-key check fails, fail before writing files.

## YAML Schema

Example:

```yaml
table: orders
resource: order
exclude_fields: []
readonly_fields:
  - id
  - created_at
  - updated_at

relations:
  - name: order_items
    table: order_items
    type: has_many
    local_field: id
    foreign_field: order_id
    methods:
      - ListOrderItemsByOrderID

  - name: user
    table: users
    type: belongs_to
    local_field: user_id
    foreign_field: id
    methods:
      - GetOrderWithUser
```

Schema fields:

- `table`: Primary table. Optional if `--table` is provided.
- `resource`: Optional logical resource name. Defaults to the service name.
- `exclude_fields`: Fields omitted from generated model, DTO, proto, request, and repository mapping.
- `readonly_fields`: Fields included in response/model/repository output but omitted from create and update inputs.
- `relations`: Associated table definitions.

Relation fields:

- `name`: Logical relation name used for generated identifiers.
- `table`: Associated table name. Must exist.
- `type`: One of `has_one`, `has_many`, or `belongs_to`.
- `local_field`: Field on the primary table. Must exist.
- `foreign_field`: Field on the associated table. Must exist.
- `methods`: Optional generated method names. If omitted, method names are derived from relation type and table names.

Field exposure is intentionally direct: the first version exposes all table fields by default, including sensitive-looking names. Users can use `exclude_fields` to remove fields.

## Database Introspection

When table-driven mode is active:

1. Load `<root>/configs/config.yaml` with the existing config loader.
2. Read `database.driver`.
3. Accept only `sqlite`, `mysql`, and `postgres`.
4. Resolve DSN:
   - SQLite: `database.dsn`
   - MySQL: `mysql.dsn`
   - PostgreSQL: `postgresql.dsn`
5. Open a database connection.
6. Validate the primary table exists.
7. Validate associated tables exist.
8. Read columns for each table:
   - Name
   - Database type
   - Nullable flag
   - Default value, when available
   - Primary-key marker
9. Validate relation fields.
10. Build template data from table metadata.

All validation must happen before writing service files.

## Type Mapping

The first version uses conservative mappings:

| Database type | Go model/repo type | Proto type |
| --- | --- | --- |
| `int`, `smallint`, `mediumint` | `int32` | `int32` |
| `bigint` | `int64` | `int64` |
| `varchar`, `text`, `char`, `uuid`, `json` | `string` | `string` |
| `bool`, `tinyint(1)` | `bool` | `bool` |
| `decimal`, `numeric` | `string` | `string` |
| `float`, `double`, `real` | `float64` | `double` |
| `date`, `datetime`, `timestamp`, `time` | `time.Time` | `string` |
| `blob`, `binary`, `bytes` | `[]byte` | `bytes` |

Naming rules:

- Database field `user_id` becomes Go field `UserID`.
- JSON and proto field names stay snake_case: `user_id`.
- Gorm tags retain the exact column name: `gorm:"column:user_id"`.
- Table names are fixed with `TableName()` and never inferred by Gorm pluralization.

Limitations:

- A primary table must have exactly one primary key field.
- Nullable fields do not become pointers in the first version. They use zero values. Users who need strict null semantics can adjust generated code.

## Generated Structure

Single-table generation keeps the existing service layout:

```text
internal/order/model/order.go
internal/order/model/repository.go
internal/order/dto/command.go
internal/order/dto/order.go
internal/order/service/service.go
internal/order/repo/gorm_repository.go
internal/order/handler/server.go
api/proto/order/v1/order.proto
internal/gateway/request/order_request.go
internal/gateway/handler/order_handler.go
internal/gateway/router/order_routes.go
```

The generated fields come from the primary table.

Multi-table generation keeps the primary table as the public resource and adds internal relation support:

```text
internal/order/model/order.go
internal/order/model/order_item.go
internal/order/model/user.go
internal/order/model/repository.go

internal/order/dto/command.go
internal/order/dto/order.go
internal/order/dto/order_item.go
internal/order/dto/user.go

internal/order/repo/gorm_repository.go
internal/order/repo/order_item_repository.go
internal/order/repo/user_repository.go
internal/order/repo/query_repository.go

internal/order/service/service.go
internal/order/service/relation_service.go
```

Public API rules:

- The primary table gets full CRUD RPC and HTTP endpoints.
- Associated tables get generated model/repo/dto code.
- Associated tables do not get standalone public CRUD RPC or HTTP endpoints.
- Relation query methods are added to the current service's proto, handler, service, repository interface, and gateway.

## Relation Query Methods

Default method naming:

- `has_many`: `List<RelatedPlural>By<MainPK>`
- `has_one`: `Get<Main>With<Related>`
- `belongs_to`: `Get<Main>With<Related>`

YAML `methods` can override generated method names.

Repository boundary:

- `model.Repository` continues to represent primary-table CRUD.
- Add `model.QueryRepository` for relation queries.
- The service depends on interfaces, not Gorm:

```go
type Service struct {
	repo    model.Repository
	queries model.QueryRepository
	log     *zap.Logger
}
```

Default query behavior:

- `has_many`: query the associated table with `WHERE foreign_field = ?`.
- `has_one`: load the primary record, then load the first associated record by relation fields.
- `belongs_to`: load the primary record, then load the associated record using the primary record's local field.

The generated implementation should favor readability and easy customization. It should not generate complex SQL joins by default because joins introduce field-name collisions and driver-specific scanning behavior.

## Error Handling

Default command:

- `bw-cli service order` keeps current behavior.

Table-driven mode:

- Missing or unsupported relational database config:
  - TTY: prompt whether to continue with default placeholder generation.
  - Non-TTY: fail before writing files.
- Database connection failure: fail before writing files.
- Primary table missing: fail before writing files.
- Associated table missing: fail before writing files.
- Relation field missing: fail before writing files.
- Primary table has no primary key: fail before writing files.
- Primary table has a composite primary key: fail before writing files.

Error messages should name the specific table and field:

```text
table "order_items" not found in configured mysql database
relation "order_items": foreign_field "order_id" not found
primary table "orders" must have exactly one primary key
```

## Testing Plan

CLI parsing:

- Existing `bw-cli service <name>` options still parse.
- `--table` is accepted.
- `--schema` is accepted.
- `--yes` is accepted.
- Command-line `--table` overrides YAML `table`.

Schema parsing:

- Valid relation schema parses.
- Missing relation table fails validation.
- Unsupported relation type fails validation.
- Missing `local_field` or `foreign_field` fails validation.

Introspection:

- SQLite temporary database table existence.
- SQLite missing table.
- Primary key recognition.
- Field type mapping.
- Relation field validation.

Generation:

- Single-table fields appear in model, DTO, proto, repository, handler, and gateway request.
- `readonly_fields` are omitted from Create/Update inputs.
- `exclude_fields` are omitted from generated output.
- Associated tables generate internal model/repo/dto files.
- Relation query methods are generated.
- Existing placeholder-template tests continue to pass.

Error paths:

- Non-TTY unavailable database config fails before writing files.
- Missing table fails before writing files.
- Missing relation field fails before writing files.

## Implementation Order

1. Extend `ServiceOptions` and CLI parsing.
2. Add schema structs and YAML parsing.
3. Add relational introspection types and SQLite implementation.
4. Add MySQL and PostgreSQL introspection implementations.
5. Convert service template data from fixed fields to field lists.
6. Update proto, model, DTO, repository, handler, gateway, and documentation templates.
7. Generate relation query interfaces and implementations.
8. Update README, `docs/usage.md`, and service documentation template.
9. Add tests and run `go test ./...`.

## Open Constraints

- First version supports only single-field primary keys.
- First version supports only single-field relations.
- First version does not generate optimized SQL joins by default.
- First version exposes all table fields unless explicitly excluded.
