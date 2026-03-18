# newapi-price-sync

`newapi-price-sync` is a standalone sidecar service that synchronizes model pricing metadata into [NewAPI](https://github.com/QuantumNous/new-api) through the shared database.

Instead of modifying NewAPI source code or calling privileged management APIs, it writes the pricing-related records directly into NewAPI's `options` table. NewAPI already reloads options from the database periodically, so updated pricing becomes effective without restarting the main service.

## What it updates

The service maintains these NewAPI option keys:

- `ModelRatio`
- `CompletionRatio`
- `CacheRatio`
- `CreateCacheRatio`
- `ModelPrice`

Each value is stored as a JSON object in the `options` table, matching NewAPI's native format.

## Supported sources

The following upstream source types are supported:

- `models_dev` — `https://models.dev/api.json`
- `openrouter` — OpenRouter `/v1/models`
- `newapi_ratio` — another NewAPI instance exposing `/api/ratio_config`
- `newapi_pricing` — another NewAPI instance exposing `/api/pricing`

Multiple sources can be configured. They are applied in order, and later sources override earlier ones for the same model field.

## How pricing is converted

This project follows NewAPI's ratio semantics:

- `1 model_ratio = $2 / 1M input tokens`

Conversion rules:

- `model_ratio = (input_price_per_1M * exchange_rate * price_multiplier) / 2`
- `completion_ratio = output_price / input_price`
- `cache_ratio = cache_read_price / input_price`
- `create_cache_ratio = cache_write_price / input_price`
- `model_price = unit_price * exchange_rate * price_multiplier`

Notes:

- `exchange_rate` and `price_multiplier` affect monetary values only, so they apply to `model_ratio` and `model_price`
- `completion_ratio`, `cache_ratio`, and `create_cache_ratio` are relative ratios and are not multiplied by exchange rate
- if an upstream source does not expose cache write pricing, existing `create_cache_ratio` values are preserved

## Merge behavior

By default, the service is conservative:

- existing database entries are preserved unless overwritten by a configured source
- models not mentioned by any configured source are left untouched
- missing fields from an upstream source do not delete existing values
- changed values are written back to the same NewAPI database

## Model name matching

Model matching is normalization-aware rather than strict string equality.

The resolver supports:

- case-insensitive matching
- provider prefix tolerance, such as `zai/`
- separator-insensitive matching across `-`, `_`, `.`, and `/`

Examples that are treated as the same model family:

- `zai/glm4.7`
- `glm4.7`
- `glm-4.7`
- `GLM4.7`

If an existing NewAPI option entry already contains one or more of these variants, the existing keys are updated in place. A new key is only created when no existing variant matches.

## Database support

Database selection is compatible with common NewAPI deployment patterns:

- empty `SQL_DSN`, or `SQL_DSN` starting with `local` → SQLite
- `SQL_DSN` starting with `postgres://` or `postgresql://` → PostgreSQL
- any other non-empty `SQL_DSN` → MySQL

SQLite defaults to:

- `/data/one-api.db`

## Configuration

Copy the example file and adjust it for your environment:

```bash
cp config.example.yaml config.yaml
```

Example:

```yaml
database:
  type: sqlite
  dsn: ""
  sqlite_path: /data/one-api.db

sources:
  - type: models_dev
    url: https://models.dev/api.json
    timeout: 30s
    enabled: true

sync:
  interval: 24h
  once: false
  dry_run: false
  preserve_unmentioned: true

currency:
  target_currency: USD
  exchange_rate: 1.0
  price_multiplier: 1.0

filter:
  include: []
  exclude: []

logging:
  level: info
  json: false
```

## Quick start

Run once:

```bash
go run ./cmd/newapi-price-sync --config ./config.yaml --once
```

Run continuously:

```bash
go run ./cmd/newapi-price-sync --config ./config.yaml
```

Dry run:

```bash
go run ./cmd/newapi-price-sync --config ./config.yaml --once --dry-run
```

Default sync interval: `24h`.

The service also performs one synchronization immediately on startup.

## Environment variables

The following environment variables override file-based configuration when present:

- `SQL_DSN`
- `SQLITE_PATH`
- `NPS_INTERVAL`
- `NPS_EXCHANGE_RATE`
- `NPS_PRICE_MULTIPLIER`
- `NPS_TARGET_CURRENCY`
- `NPS_DRY_RUN`
- `NPS_ONCE`

## Docker

### Build

```bash
docker build -t newapi-price-sync:local .
```

### Example: SQLite-based NewAPI

```yaml
services:
  newapi:
    image: quantumnous/new-api:latest
    volumes:
      - ./data:/data

  newapi-price-sync:
    image: newapi-price-sync:local
    restart: unless-stopped
    environment:
      SQLITE_PATH: /data/one-api.db
      NPS_TARGET_CURRENCY: USD
      NPS_EXCHANGE_RATE: "1"
      NPS_PRICE_MULTIPLIER: "1"
    volumes:
      - ./data:/data
      - ./config.yaml:/app/config.yaml:ro
    depends_on:
      - newapi
```

### Example: MySQL or PostgreSQL-based NewAPI

```yaml
services:
  newapi:
    image: quantumnous/new-api:latest
    environment:
      SQL_DSN: postgresql://root:password@postgres:5432/new-api

  newapi-price-sync:
    image: newapi-price-sync:local
    restart: unless-stopped
    environment:
      SQL_DSN: postgresql://root:password@postgres:5432/new-api
      NPS_TARGET_CURRENCY: USD
      NPS_EXCHANGE_RATE: "1"
      NPS_PRICE_MULTIPLIER: "1"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
    depends_on:
      - newapi
```

## Filtering models

Model filters are regular-expression based:

```yaml
filter:
  include:
    - '^gpt-'
    - '^claude-'
  exclude:
    - 'vision'
```

## Current limitations

- upstreams that do not expose cache write pricing cannot generate `create_cache_ratio`
- `newapi_ratio` is most useful when another NewAPI deployment is already maintained as a pricing source
- `newapi_pricing` keeps NewAPI-style ratio semantics for `quota_type=0` models and only scales the monetary base

## Repository structure

```text
cmd/newapi-price-sync/     application entrypoint
internal/config/           configuration loading
internal/db/               database integration
internal/fetcher/          upstream source fetchers
internal/merger/           merge and alias resolution logic
internal/models/           shared data structures
pkg/normalize/             pricing normalization helpers
```

## License

MIT
