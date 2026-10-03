# OFM Auth Service

## Purpose

The Auth Service owns authentication credentials and email-verification state. It is the source of truth for password hashes, auth identities, and verification codes. Status: active.

## Boundaries and flow

Registration saga commands arrive through NATS. The service validates the command, writes credentials and verification state to PostgreSQL, commits its outbox record, and publishes the result for the saga and mail service. It does not own user profiles or registration workflow state.

CDC reads committed outbox changes for Kafka publication and migration projections; processing is idempotent by event identity. Other services use the auth contract rather than its tables.

## Configuration

.env.example groups are DB_*, MIGRATIONS_*, NATS_*, listener settings, JWT/auth settings, and observability. Database values select auth storage; NATS values select command subjects and durable consumers; listener values select gRPC and metrics ports. Secrets remain local.

## Local development

Run from this repository:

    cp .env.example .env
    just run
    go test ./...

## Build and operations

Dockerfile builds ofm/auth-service:<tag> and Helm deploys the workload. Use auth traces, structured logs, NATS consumer state, Kafka lag, and projection audit records when diagnosing registration failures.

