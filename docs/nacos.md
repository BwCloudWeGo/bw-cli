# Nacos 配置中心使用说明

本文说明如何在不修改业务配置结构体的前提下启用 Nacos 配置中心。

## 1. 配置来源规则

项目启动时会先读取 `configs/nacos.yaml`：

- `enabled: false`：默认模式，读取本地 `configs/config.yaml`。
- `enabled: true`：配置中心模式，从 Nacos 拉取完整业务配置，作为整套系统配置。

```text
默认模式：
默认值 -> configs/config.yaml

Nacos 模式：
默认值 -> Nacos 中的完整 YAML 配置
```

`configs/nacos.yaml` 只负责控制是否启用 Nacos，以及如何连接 Nacos。不要把业务配置项写进 `configs/nacos.yaml`。

## 2. Nacos 开关配置

默认文件：

```yaml
enabled: false
server_addr: 127.0.0.1
server_port: 8848
namespace_id: ""
group: DEFAULT_GROUP
data_id: xiaolanshu.yaml
username: ""
password: ""
timeout_ms: 5000
log_dir: logs/nacos
cache_dir: data/nacos/cache
log_level: info
fail_fast: false
watch: false
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `enabled` | 是否启用 Nacos。默认 `false`。 |
| `server_addr` | Nacos 服务地址，不带协议。 |
| `server_port` | Nacos 服务端口，默认 `8848`。 |
| `namespace_id` | Nacos 命名空间 ID。默认空字符串，表示 public。 |
| `group` | 配置分组，默认 `DEFAULT_GROUP`。 |
| `data_id` | Nacos 配置 Data ID，建议用 `xiaolanshu.yaml`。 |
| `username` / `password` | Nacos 开启鉴权时填写。 |
| `timeout_ms` | 拉取配置超时时间，单位毫秒。 |
| `log_dir` | Nacos SDK 日志目录。 |
| `cache_dir` | Nacos SDK 缓存目录。 |
| `log_level` | Nacos SDK 日志级别。 |
| `fail_fast` | 拉取失败时是否启动失败。生产建议 `true`。 |
| `watch` | 预留开关。当前启动期拉取配置，不自动热重建服务。 |

## 3. 在 Nacos 中创建配置

进入 Nacos 控制台，创建配置：

```text
Data ID: xiaolanshu.yaml
Group:   DEFAULT_GROUP
Format:  YAML
```

配置内容直接复制 `configs/config.yaml` 的完整内容即可。

示例：

```yaml
app:
  name: xiaolanshu
  env: local

http:
  host: 0.0.0.0
  port: 8080
  read_timeout_seconds: 5
  write_timeout_seconds: 10

grpc:
  host: 0.0.0.0

services:
  gateway:
    name: gateway
  user:
    name: user-service
    port: 9001
    target: 127.0.0.1:9001
  note:
    name: note-service
    port: 9002
    target: 127.0.0.1:9002
```

实际使用时应复制完整的 `configs/config.yaml`，包括数据库、Redis、Kafka、JWT、日志等配置。

## 4. 启用 Nacos

修改 `configs/nacos.yaml`：

```yaml
enabled: true
server_addr: 127.0.0.1
server_port: 8848
namespace_id: ""
group: DEFAULT_GROUP
data_id: xiaolanshu.yaml
username: ""
password: ""
timeout_ms: 5000
log_dir: logs/nacos
cache_dir: data/nacos/cache
log_level: info
fail_fast: true
watch: false
```

然后正常启动服务：

```bash
make run-user
make run-note
make run-gateway
```

启动后，应用不会再使用 `configs/config.yaml` 作为业务配置来源，而是使用 Nacos 中 `data_id` 对应的完整 YAML 配置。

## 5. 本地开发和生产建议

本地开发建议：

```yaml
enabled: false
fail_fast: false
```

这样不依赖 Nacos，直接使用 `configs/config.yaml`。

生产环境建议：

```yaml
enabled: true
fail_fast: true
```

这样 Nacos 拉取失败时服务直接启动失败，避免使用错误的本地兜底配置悄悄启动。

## 6. 不需要修改 Config 结构体

Nacos 中存放的是和 `configs/config.yaml` 同结构的完整 YAML。

因此，新增业务配置时只需要：

1. 在 `configs/config.yaml` 增加字段。
2. 在对应的 Go 配置结构体中增加字段。
3. 把新的完整 `configs/config.yaml` 内容复制到 Nacos。

启用 Nacos 不需要为远程配置额外定义一套结构体，也不需要改业务读取配置的代码。

## 7. 排查

如果启用 Nacos 后配置没有生效，按顺序检查：

1. `configs/nacos.yaml` 中 `enabled` 是否为 `true`。
2. `data_id` 和 `group` 是否与 Nacos 控制台一致。
3. `namespace_id` 是否填的是命名空间 ID，不是命名空间名称。
4. Nacos 配置格式是否选择 YAML。
5. Nacos 中的内容是否是完整业务配置，而不是只写了部分字段。
6. Nacos 中的 YAML 结构是否与本地 `configs/config.yaml` 保持一致。

如果生产环境不允许本地兜底，把 `fail_fast` 设置为 `true`。
