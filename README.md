# OFM Auth Service

## Purpose

`ofm-auth-service` owns authentication data and email verification state.
It is the source of truth for:

- auth credentials
- password hashes
- email verification codes

In the current registration flow it consumes saga commands, creates auth
credentials, generates and persists verification codes, and publishes the mail
send command used by `mail-service`.

## Run

Local process:

```bash
cp .env.example .env
just run
```

Direct Go command:

```bash
set -a && source .env && set +a && go run ./cmd/auth-service
```

Docker stack from the shared infra repo:

```bash
cd ../ofm-infra
just infra-up
```

## Environment

```env
APP_ENV=local
LOG_LEVEL=info

DB_HOST=127.0.0.1
DB_PORT=5434
DB_USER=yugabyte
DB_PASSWORD=yugabyte
DB_NAME=auth_service
DB_SSLMODE=disable
DB_MAX_OPEN_CONNS=20
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME=5m

MIGRATIONS_PATH=file://migration/yugabyte
MIGRATIONS_TABLE=schema_migrations_auth_service

NATS_URL=nats://127.0.0.1:4222
NATS_USER=
NATS_PASSWORD=
NATS_STREAM_AUTH_EVENTS=AUTH_EVENTS
NATS_STREAM_MAIL_COMMANDS=MAIL_COMMANDS
NATS_STREAM_SAGA_COMMANDS=SAGA_AUTH_COMMANDS
NATS_SUBJECT_AUTH_CREATED=auth.created
NATS_SUBJECT_SAGA_CREATE_PENDING_AUTH=saga.auth.create_pending_registration
NATS_SUBJECT_SAGA_DELETE_AUTH=saga.auth.delete
NATS_SUBJECT_SAGA_CREATE_PENDING_AUTH_RESULT=saga.auth.create_pending_registration.result
NATS_SUBJECT_SAGA_DELETE_AUTH_RESULT=saga.auth.delete.result
NATS_SUBJECT_MAIL_SEND=mail.send
NATS_DURABLE_SAGA_CREATE_AUTH=auth_service_saga_create_pending_registration
NATS_DURABLE_SAGA_DELETE_AUTH=auth_service_saga_delete
NATS_SAGA_BATCH_SIZE=32
NATS_SAGA_MAX_WAIT=10ms
NATS_SAGA_WORKERS=8
NATS_SAGA_QUEUE_SIZE=500
NATS_SAGA_ACK_WAIT=30s
NATS_SAGA_MAX_DELIVER=5
NATS_SAGA_ADAPTIVE_ENABLED=false
NATS_SAGA_ADAPTIVE_CHECK_INTERVAL=2s
NATS_SAGA_ADAPTIVE_MEDIUM_PENDING=200
NATS_SAGA_ADAPTIVE_HIGH_PENDING=1000
NATS_SAGA_ADAPTIVE_LOW_BATCH_SIZE=8
NATS_SAGA_ADAPTIVE_LOW_MAX_WAIT=25ms
NATS_SAGA_ADAPTIVE_MEDIUM_BATCH_SIZE=32
NATS_SAGA_ADAPTIVE_MEDIUM_MAX_WAIT=10ms
NATS_SAGA_ADAPTIVE_HIGH_BATCH_SIZE=128
NATS_SAGA_ADAPTIVE_HIGH_MAX_WAIT=2ms
```

## Technologies

Core runtime:

- Go
- YugabyteDB/Postgres wire protocol for write-model persistence
- NATS JetStream for saga command and result transport
- Uber Fx for wiring
- Zap for structured logging
- SQL migrations via `golang-migrate`

Main libraries from `go.mod`:

- `github.com/jmoiron/sqlx`
- `github.com/jackc/pgx/v5`
- `github.com/golang-migrate/migrate/v4`
- `github.com/nats-io/nats.go`
- `go.uber.org/fx`
- `go.uber.org/zap`
- `github.com/google/uuid`

## Architecture Notes

- `internal/domain` defines auth entities and business errors
- `internal/application` owns credential creation and verification code
  generation
- `internal/infra/write/yugabyte` owns storage models and queries
- `internal/presentation/event_broker/nats` owns saga command subscribers
- `migration/yugabyte` contains schema migrations

This service should remain the owner of verification-code semantics. The
registration saga can orchestrate the flow, but it should not become the source
of truth for auth data.
