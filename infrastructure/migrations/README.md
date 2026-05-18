# infrastructure/migrations/README.md
# Migration Files

This directory contains SQL migration files managed by [golang-migrate](https://github.com/golang-migrate/migrate).

## Naming Convention

```
{sequence}_{description}.{direction}.sql

sequence:    3-digit zero-padded number (000, 001, 002, ...)
description: snake_case verb + object (create_routes_table)
direction:   up | down
```

## Examples

```
000_enable_postgis.up.sql
000_enable_postgis.down.sql
001_create_routes_table.up.sql
001_create_routes_table.down.sql
```

## Commands

```bash
# Run all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check current version
make migrate-version

# Create new migration
make migrate-create NAME=create_users_table
```

## Manual Commands (without Make)

```bash
# Set DATABASE_URL first
export DATABASE_URL=postgres://alebus:alebus@localhost:5432/alebus?sslmode=disable

# Run migrations
migrate -path infrastructure/migrations -database "$DATABASE_URL" up

# Rollback
migrate -path infrastructure/migrations -database "$DATABASE_URL" down 1

# Check version
migrate -path infrastructure/migrations -database "$DATABASE_URL" version
```
