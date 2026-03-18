# newapi-price-sync

一个给 NewAPI 用的旁路同步器。它不改 NewAPI 源码，也不走 NewAPI 管理接口鉴权。
它直接连同一个数据库，把 `options` 表里的这些键定期更新掉：

- `ModelRatio`
- `CompletionRatio`
- `CacheRatio`
- `CreateCacheRatio`
- `ModelPrice`

NewAPI 本身会周期性从数据库重新加载 options，所以这个 sidecar 写库后，主服务不用重启。

## 我根据 NewAPI 源码确认过的点

NewAPI 把倍率和按次价格都放在 `options` 表里，值是 JSON 字符串。
对应热加载入口是 `SyncOptions`，所以只要写同一个库就能生效。

NewAPI 的核心计费语义里，`model_ratio` 对应的是输入价格换算后的倍率：

- 1 个倍率单位 = `$2 / 1M input tokens`
- 所以官方输入价如果是 `$2.5 / 1M`，对应 `model_ratio = 1.25`

这个程序按下面规则换算：

- `model_ratio = (input_price_per_1M * exchange_rate * price_multiplier) / 2`
- `completion_ratio = output_price / input_price`
- `cache_ratio = cache_read_price / input_price`
- `create_cache_ratio = cache_write_price / input_price`
- `model_price = unit_price * exchange_rate * price_multiplier`

注意：

- `exchange_rate` 和 `price_multiplier` 只影响真正的价格基数，也就是 `model_ratio` / `model_price`
- `completion_ratio` / `cache_ratio` / `create_cache_ratio` 是相对倍率，不跟汇率一起放大
- 如果上游没给 `create_cache_ratio`，程序会保留数据库里原来的值，不会清空

## 支持的数据源

- `models_dev`：`https://models.dev/api.json`
- `openrouter`：`/v1/models`
- `newapi_ratio`：另一套 NewAPI 的 `/api/ratio_config`
- `newapi_pricing`：另一套 NewAPI 的 `/api/pricing`

多个 source 按配置顺序依次合并，**后面的覆盖前面的**。

模型名匹配不是死板精确匹配。现在会额外做一层归一化匹配：

- 忽略大小写
- 忽略提供商前缀（如 `zai/`）
- 忽略常见分隔符差异（`-`、`_`、`.`、`/`）

比如 `zai/glm4.7` 会和 `glm4.7`、`glm-4.7`、`GLM4.7` 视为同一模型。

## 配置

先复制：

```bash
cp config.example.yaml config.yaml
```

### USD 1:1 真价消耗

```yaml
currency:
  target_currency: USD
  exchange_rate: 1.0
  price_multiplier: 1.0
```

这时如果上游价格本来就是美元官方价，写进 NewAPI 的倍率就是 1:1 真实消耗。

### 自定义汇率 / 加价倍率

```yaml
currency:
  target_currency: CNY
  exchange_rate: 7.2
  price_multiplier: 1.15
```

这表示先按 7.2 汇率折算，再整体乘 1.15。

## 运行

### 本地一次执行

```bash
go run ./cmd/newapi-price-sync --config ./config.yaml --once
```

### 常驻循环

```bash
go run ./cmd/newapi-price-sync --config ./config.yaml
```

默认同步周期是 `24h`。

### Dry run

```bash
go run ./cmd/newapi-price-sync --config ./config.yaml --once --dry-run
```

## 环境变量覆盖

- `SQL_DSN`
- `SQLITE_PATH`
- `NPS_INTERVAL`
- `NPS_EXCHANGE_RATE`
- `NPS_PRICE_MULTIPLIER`
- `NPS_TARGET_CURRENCY`
- `NPS_DRY_RUN`
- `NPS_ONCE`

数据库类型和 NewAPI 约定保持一致：

- `SQL_DSN` 为空，或以 `local` 开头：走 SQLite
- `SQL_DSN` 以 `postgres://` 或 `postgresql://` 开头：走 PostgreSQL
- 其他非空 `SQL_DSN`：走 MySQL

## Docker Compose 接法

### SQLite 场景

假设你的 NewAPI 把 sqlite 文件挂在 `/data/one-api.db`：

```yaml
services:
  newapi:
    image: quantumnous/new-api:latest
    volumes:
      - ./data:/data

  newapi-price-sync:
    build: ./newapi-price-sync
    container_name: newapi-price-sync
    restart: unless-stopped
    environment:
      SQLITE_PATH: /data/one-api.db
      NPS_TARGET_CURRENCY: USD
      NPS_EXCHANGE_RATE: "1"
      NPS_PRICE_MULTIPLIER: "1"
    volumes:
      - ./data:/data
      - ./newapi-price-sync/config.yaml:/app/config.yaml:ro
    depends_on:
      - newapi
```

### MySQL / PostgreSQL 场景

直接复用 NewAPI 的数据库环境：

```yaml
services:
  newapi:
    image: quantumnous/new-api:latest
    environment:
      SQL_DSN: root:password@tcp(mysql:3306)/newapi

  newapi-price-sync:
    build: ./newapi-price-sync
    container_name: newapi-price-sync
    restart: unless-stopped
    environment:
      SQL_DSN: root:password@tcp(mysql:3306)/newapi
      NPS_TARGET_CURRENCY: USD
      NPS_EXCHANGE_RATE: "1"
      NPS_PRICE_MULTIPLIER: "1"
    volumes:
      - ./newapi-price-sync/config.yaml:/app/config.yaml:ro
    depends_on:
      - newapi
```

## 过滤模型

```yaml
filter:
  include:
    - '^gpt-'
    - '^claude-'
  exclude:
    - 'vision'
```

## 现在这版的边界

- `models.dev` / `openrouter` 如果没有暴露 cache write 价格，就不会生成 `create_cache_ratio`
- `newapi_ratio` 适合拿另一套已经配好的 NewAPI 当“价格源”
- `newapi_pricing` 里 `quota_type=0` 的条目会按 NewAPI 语义继续保留倍率，只对货币基数做缩放

如果你要，我下一步可以直接把它整理到你工作区，顺手再给你一份能直接塞进现有 `docker-compose.yml` 的 service 片段。
