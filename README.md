# Nerve Platform Startup Guide

This document provides a step-by-step guide to start and run the Nerve exchange platform using Turborepo, Docker, Kafka, Redis, ClickHouse, and Postgres.

---

## 1. Prerequisites

- Node.js >= 18
- PNPM >= 9.0.0
- Docker & Docker Compose
- Turbo CLI (installed globally if needed)
- (Optional) curl for health checks

---

## 2. Install Dependencies

```bash
# Install dependencies for all apps and packages
pnpm install

# Install Husky hooks
pnpm prepare
```

---

## 3. Start Infrastructure

Navigate to the infra folder and bring up all services.

```bash
# Start all containers (Postgres, Redis, Kafka, Zookeeper, ClickHouse)
pnpm run infra:up

# View logs
pnpm run infra:logs

# To view logs for specific service
pnpm run infra:logs:clickhouse
pnpm run infra:logs:postgres
pnpm run infra:logs:redis
pnpm run infra:logs:kafka
pnpm run infra:logs:zookeeper
```

### Reset Infrastructure

```bash
# Stops, removes volumes, and starts again
pnpm run infra:reset

# Restart containers
pnpm run infra:restart

# Check running containers
pnpm run infra:ps
```

---

## 4. Check Database Health

Ensure all services are up and running.

```bash
# Check ClickHouse
pnpm run db:health:clickhouse

# Check Postgres
pnpm run db:health:postgres

# Check Redis
pnpm run db:health:redis

# Check Kafka
pnpm run db:health:kafka

# Check all
pnpm run db:health:all
```

---

## 5. Connect to Databases

```bash
# ClickHouse
pnpm run db:connect:clickhouse

# Postgres
pnpm run db:connect:postgres

# Redis
pnpm run db:connect:redis
```

---

## 6. Manage Kafka Topics

```bash
# List topics
pnpm run kafka:topics:list

# Create a new topic
pnpm run kafka:topics:create <topic_name>

# Start a consumer
pnpm run kafka:consumer <topic_name>

# Start a producer
pnpm run kafka:producer <topic_name>
```

---

## 7. Run Applications

### Start in Development Mode

```bash
# Start all apps in dev mode
pnpm run dev
```

### Build Applications

```bash
# Build all apps and packages
pnpm run build

# Build logger package separately
pnpm run build:logger
```

### Lint and Format Code

```bash
# Lint all apps and packages
pnpm run lint
pnpm run lint:fix

# Format code
pnpm run format
pnpm run format:check
```

### Type Checking

```bash
pnpm run check-types
```

---

## 8. Notes

- Ensure `.env` files are set correctly in each app/package.
- Follow the order flow documentation for API usage.
- Kafka and Redis must be running before starting services.
- Use WebSocket events for real-time updates.

---

**End of Startup Guide**
