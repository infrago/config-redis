# config-redis

`config-redis` 是 `config` 模块的 `redis` 驱动。

## 安装

```bash
go get github.com/infrago/config@latest
go get github.com/infrago/config-redis@latest
```

## 接入

```go
import (
    _ "github.com/infrago/config"
    _ "github.com/infrago/config-redis"
    "github.com/infrago/infra"
)

func main() {
    infra.Run()
}
```

## 配置示例

```toml
[config]
driver = "redis"
```

## 公开 API（摘自源码）

- `func (d *redisConfigDriver) Load(params base.Map) (base.Map, error)`

## 排错

- driver 未生效：确认模块段 `driver` 值与驱动名一致
- 连接失败：检查 endpoint/host/port/鉴权配置
