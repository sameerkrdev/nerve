# Nerve Platform — Setup Guide

Full local development setup from scratch.

---

## Prerequisites

| Tool                    | Version  | Notes                    |
| ----------------------- | -------- | ------------------------ |
| Node.js                 | >= 18    | `node -v` to verify      |
| pnpm                    | >= 9.0.0 | `npm i -g pnpm`          |
| Go                      | >= 1.23  | Required for Go services |
| Docker + Docker Compose | v2+      | For infra                |
| Git                     | any      | —                        |

---

## Repo Structure

```
nerve/
  apps/
    api-gateway/           TS — HTTP entry point (port 8001)
    auth-service/          TS — gRPC auth server (port 50054)
    order-service/         TS — gRPC order server (port 50051)
    order-trade-store-service/  TS — Kafka consumer → Postgres
    market-maker/          TS — stub
    matching-engine/       Go — gRPC matching + Redis pub/sub (port 50052)
    candle-service/        Go — Kafka consumer → candles + gRPC (port 50054*)
    trade-ingestor-service/ Go — Kafka consumer → ClickHouse
    websocket-server/      Go — WS server + Redis fan-out (port 50053)
    web/                   React — frontend scaffold
  packages/
    prisma/                Shared Prisma client + schema
    proto-defs/            .proto files + generated TS/Go code
    validator/             Zod schemas
    logger/                Shared logger
    kafka-client/          Shared Kafka producer/consumer helpers
    types/                 Shared TypeScript types
  infra/
    docker/
      db/                  Redis + PostgreSQL compose
      kafka/               3-broker KRaft Kafka compose
      clickhouse/          ClickHouse compose
```

> \* candle-service and auth-service both default to port 50054 in their `.env.example`. Run them on separate machines or adjust one port.

---

## Step 1 — Clone & Install

```bash
git clone <repo-url>
cd nerve

# Install all Node packages (workspaces)
pnpm install

# Install Husky git hooks
pnpm prepare
```

---

## Step 2 — Generate RSA Key Pair (one-time)

Auth-service signs JWTs with RS256. api-gateway and websocket-server verify with the public key.

```bash
# Generate keys
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem

# Base64-encode (Linux/macOS)
base64 -w 0 private.pem   # → paste as JWT_PRIVATE_KEY
base64 -w 0 public.pem    # → paste as JWT_PUBLIC_KEY

# Base64-encode (Windows PowerShell)
[Convert]::ToBase64String([IO.File]::ReadAllBytes("private.pem"))
[Convert]::ToBase64String([IO.File]::ReadAllBytes("public.pem"))
```

Keep the `JWT_PRIVATE_KEY` **only** in `auth-service/.env`. Distribute `JWT_PUBLIC_KEY` to api-gateway and websocket-server.

Delete `private.pem` and `public.pem` after copying into env files.

---

## Step 3 — Configure Environment Files

Copy each `.env.example` to `.env` and fill in values.

### Root `.env`

```bash
cp .env.example .env
```

```env
COMPOSE_PROJECT_NAME=nerve

# ClickHouse
CLICKHOUSE_DB=nerve
CLICKHOUSE_USER=nerve
CLICKHOUSE_PASSWORD=nerve

# PostgreSQL
POSTGRES_DB=nerve
POSTGRES_USER=nerve
POSTGRES_PASSWORD=nerve
```

---

### `apps/auth-service/.env`

```env
AUTH_SERVICE_GRPC_URL=0.0.0.0:50054
NODE_ENV=development

POSTGRES_DATABASE_URL=postgresql://nerve:nerve@localhost:5432/nerve

REDIS_URL=redis://localhost:6380

JWT_PRIVATE_KEY=<base64-encoded private.pem>
JWT_PUBLIC_KEY=<base64-encoded public.pem>

ACCESS_TOKEN_EXPIRY=900        # 15 minutes
REFRESH_TOKEN_EXPIRY=604800    # 7 days
```

---

### `apps/api-gateway/.env`

```env
PORT=8001
NODE_ENV=development

ORDER_SERVICE_GRPC_URL=localhost:50051
AUTH_SERVICE_GRPC_URL=localhost:50054

JWT_PUBLIC_KEY=<base64-encoded public.pem>

REDIS_URL=redis://localhost:6380
```

---

### `apps/order-service/.env`

```env
NODE_ENV=development
ORDER_SERVICE_GRPC_URL=localhost:50051
MATCHING_SERVICE_GRPC_URL=localhost:50052
```

---

### `apps/order-trade-store-service/.env`

```env
NODE_ENV=development
ORDER_SERVICE_GRPC_URL=localhost:50051
KAFKA_BROKERS=localhost:19092,localhost:19093,localhost:19094
```

---

### `apps/matching-engine/.env`

```env
PORT=50052
KAFKA_BROKERS=localhost:19092,localhost:19093,localhost:19094
REDIS_URL=redis://localhost:6380
```

---

### `apps/candle-service/.env`

```env
PORT=50054
KAFKA_BROKERS=localhost:19092,localhost:19093,localhost:19094
REDIS_URL=redis://localhost:6380

CLICKHOUSE_ADDR=localhost:9000
CLICKHOUSE_DATABASE=nerve
CLICKHOUSE_USER=nerve
CLICKHOUSE_PASSWORD=nerve
```

> Note: if auth-service is also on 50054, move candle-service to a different port (e.g. 50055) and update any callers.

---

### `apps/trade-ingestor-service/.env`

```env
KAFKA_BROKERS=localhost:19092,localhost:19093,localhost:19094

CLICKHOUSE_ADDR=localhost:9000
CLICKHOUSE_DATABASE=nerve
CLICKHOUSE_USER=nerve
CLICKHOUSE_PASSWORD=nerve
```

---

### `apps/websocket-server/.env`

```env
PORT=50053
REDIS_URL=redis://localhost:6380
JWT_PUBLIC_KEY=<base64-encoded public.pem>
NODE_ENV=development
```

---

### `packages/prisma/.env`

```env
DATABASE_URL=postgresql://nerve:nerve@localhost:5432/nerve
```

---

## Step 4 — Start Infrastructure

```bash
# Start Redis, PostgreSQL, Kafka (3-broker KRaft), ClickHouse
pnpm run infra:up

# Verify all containers are running
pnpm run infra:ps

# Check health
pnpm run db:health:all
```

### Infrastructure ports

| Service           | Port  | Notes                               |
| ----------------- | ----- | ----------------------------------- |
| Redis             | 6380  | Non-default port to avoid conflicts |
| PostgreSQL        | 5432  | User/pass/db = nerve/nerve/nerve    |
| Kafka broker 1    | 19092 | External listener                   |
| Kafka broker 2    | 19093 | External listener                   |
| Kafka broker 3    | 19094 | External listener                   |
| Kafka UI          | 8080  | Browser: `http://localhost:8080`    |
| ClickHouse HTTP   | 8123  | REST queries                        |
| ClickHouse native | 9000  | Go/native client                    |

---

## Step 5 — Database Setup

### PostgreSQL — Prisma migrations

```bash
cd packages/prisma
pnpm prisma migrate deploy
# or for dev (generates migration files)
pnpm prisma migrate dev
```

### ClickHouse — schema auto-created

`trade-ingestor-service` calls `EnsureTradesSchema()` on startup — no manual migration needed.

---

## Step 6 — Kafka Topics

Kafka topics are created by a startup script inside the compose setup. Verify they exist:

```bash
pnpm run kafka:topics:list
```

Expected topics: `engine-events`, `trades`, `candles`.

Create manually if missing:

```bash
pnpm run kafka:topics:create engine-events
pnpm run kafka:topics:create trades
pnpm run kafka:topics:create candles
```

---

## Step 7 — Start All Services

```bash
# Start all apps in parallel (Turborepo)
pnpm run dev
```

Or start individually for debugging:

```bash
# Node services (from their app directory)
cd apps/auth-service && pnpm dev
cd apps/api-gateway && pnpm dev
cd apps/order-service && pnpm dev
cd apps/order-trade-store-service && pnpm dev

# Go services (uses air for hot reload)
cd apps/matching-engine && air
cd apps/candle-service && air
cd apps/trade-ingestor-service && air
cd apps/websocket-server && air
```

### Start order matters

Services have dependencies. Suggested order:

```
1. Infrastructure (Redis, Postgres, Kafka, ClickHouse)   ← Step 4
2. auth-service          (no service deps)
3. order-service         (no service deps)
4. matching-engine       (needs Kafka + Redis)
5. candle-service        (needs Kafka + Redis + ClickHouse)
6. trade-ingestor-service (needs Kafka + ClickHouse)
7. order-trade-store-service (needs Kafka + order-service)
8. websocket-server      (needs Redis)
9. api-gateway           (needs auth-service + order-service + Redis)
```

---

## Step 8 — Verify Services

| Service          | URL / Command                           | Expected                                                           |
| ---------------- | --------------------------------------- | ------------------------------------------------------------------ |
| api-gateway      | `curl http://localhost:8001/auth/me`    | `401 Unauthorized` (not `connection refused`)                      |
| websocket-server | `curl http://localhost:50053/api/v1/ws` | `400 Bad Request` (WS upgrade required — not `connection refused`) |
| Redis            | `pnpm run db:health:redis`              | `PONG`                                                             |
| Kafka            | `pnpm run db:health:kafka`              | broker list                                                        |
| ClickHouse       | `pnpm run db:health:clickhouse`         | `200 OK`                                                           |
| Postgres         | `pnpm run db:health:postgres`           | `accepting connections`                                            |

---

## Step 9 — Import Postman Collection

1. Open Postman
2. **Import** → select `docs/nerve.postman_collection.json`
3. Set collection variable `baseUrl` = `http://localhost:8001`
4. Run **Auth → Register** to create an account — tokens are auto-saved
5. All protected routes pick up `{{accessToken}}` automatically

For WebSocket testing in Postman:

1. **New** → **WebSocket Request**
2. URL: `ws://localhost:50053/api/v1/ws?token={{accessToken}}`
3. **Connect**
4. Send subscribe messages (text frame, JSON):

```json
{ "action": "subscribe_depth", "symbol": "BTCUSD" }
```

---

## Troubleshooting

### `go: cannot load module ... go.mod: file not found`

Run `go work sync` from repo root, or manually create the missing `go.mod` inside `packages/proto-defs/go/generated/`.

### `pnpm install` fails on `@repo/proto-defs`

Proto TS files must be generated first:

```bash
cd packages/proto-defs && pnpm proto:gen
```

Requires `protoc` and `protoc-gen-ts` to be installed.

### Port 50054 conflict (auth-service vs candle-service)

Both default to 50054. Change candle-service `PORT` to 50055 in its `.env`.

### Kafka connection refused

Ensure infra is up (`pnpm run infra:up`) and brokers are healthy before starting engine/candle/ingestor services.

### Redis `WRONGPASS` or connection refused

Check `REDIS_URL` in each `.env` — port is 6380, not 6379.

### JWT_PUBLIC_KEY env missing in api-gateway / websocket-server

Auth middleware will fail to start. Verify the base64-encoded public key is set (no newlines in the base64 string).

### Go services fail to build: `wsg.redisClient undefined`

Field was renamed to `redis` in the websocket-server refactor. Pull latest.

---

## Service Port Reference

| Service          | Protocol | Port    |
| ---------------- | -------- | ------- |
| api-gateway      | HTTP     | 8001    |
| auth-service     | gRPC     | 50054   |
| order-service    | gRPC     | 50051   |
| matching-engine  | gRPC     | 50052   |
| candle-service   | gRPC     | 50054\* |
| websocket-server | HTTP/WS  | 50053   |

---

## Key Docs

| Doc                                  | What it covers                                                 |
| ------------------------------------ | -------------------------------------------------------------- |
| `README.md`                          | pnpm scripts reference for infra, Kafka, dev commands          |
| `docs/PLACE_ORDER_DATA_FLOW.md`      | End-to-end order lifecycle, service diagram, Kafka topics      |
| `docs/REALTIME_EVENT_FLOW.md`        | WebSocket protocol, event types, Redis keys, fan-out internals |
| `docs/nerve.postman_collection.json` | Import this into Postman                                       |
| `PROGRESS.md`                        | Build status for all services                                  |
