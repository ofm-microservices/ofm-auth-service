package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	auth "auth-service/internal/domain"

	"github.com/jmoiron/sqlx"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRepositoryBranches(t *testing.T) {
	t.Run("verify registration email success and scan failure", func(t *testing.T) {
		db, cleanup := newFakeSQLDB(t, fakeDBConfig{
			queryRow: func(query string, args []driver.Value) fakeRowResult {
				switch {
				case strings.Contains(query, "UPDATE email_verification_codes"):
					if len(args) > 0 && args[0] == "missing" {
						return fakeRowResult{err: sql.ErrNoRows}
					}
					return fakeRowResult{
						columns: []string{"user_id", "email", "username", "password_hash", "email_verified", "status", "created_at", "updated_at"},
						values: []driver.Value{"user-1", "user@example.com", "tester", "hash", true, "email_verified", time.Now(), time.Now()},
					}
				default:
					return fakeRowResult{err: sql.ErrNoRows}
				}
			},
		})
		defer cleanup()

		repo, err := New(db, NewPgErrorTranslator())
		if err != nil {
			t.Fatalf("new repo: %v", err)
		}

		cred, err := repo.VerifyRegistrationEmail(context.Background(), "user-1", "token", time.Now())
		if err != nil {
			t.Fatalf("verify registration email: %v", err)
		}
		if cred.UserID != "user-1" || cred.Email != "user@example.com" || !cred.EmailVerified {
			t.Fatalf("unexpected credential: %+v", cred)
		}

		_, err = repo.VerifyRegistrationEmail(context.Background(), "missing", "token", time.Now())
		if !errors.Is(err, auth.ErrInvalidVerificationCode) {
			t.Fatalf("expected invalid verification code, got %v", err)
		}
	})

	t.Run("refresh token and deactivate/delete branches", func(t *testing.T) {
		db, cleanup := newFakeSQLDB(t, fakeDBConfig{
			queryRow: func(query string, args []driver.Value) fakeRowResult {
				switch {
				case strings.Contains(query, "SELECT user_id, email, username"):
					return fakeRowResult{
						columns: []string{"user_id", "email", "username", "password_hash", "email_verified", "status", "created_at", "updated_at"},
						values: []driver.Value{"user-2", "user2@example.com", "tester", "hash", true, "email_verified", time.Now(), time.Now()},
					}
				case strings.Contains(query, "SELECT EXISTS"):
					return fakeRowResult{columns: []string{"exists"}, values: []driver.Value{true}}
				default:
					return fakeRowResult{err: sql.ErrNoRows}
				}
			},
			exec: func(query string, args []driver.Value) fakeExecResult {
				switch {
				case strings.Contains(query, "INSERT INTO refresh_tokens"):
					if len(args) > 1 && args[1] == "not-a-uuid" {
						return fakeExecResult{err: &pgconn.PgError{Code: pgerrcode.InvalidTextRepresentation}}
					}
					return fakeExecResult{result: fakeResult{rows: 1}}
				case strings.Contains(query, "UPDATE auth_credentials"):
					if len(args) > 0 && args[0] == "missing-user" {
						return fakeExecResult{result: fakeResult{rows: 0}}
					}
					return fakeExecResult{result: fakeResult{rows: 1}}
				case strings.Contains(query, "DELETE FROM auth_credentials"):
					if len(args) > 0 && args[0] == "missing-user" {
						return fakeExecResult{result: fakeResult{rows: 0}}
					}
					return fakeExecResult{result: fakeResult{rows: 1}}
				default:
					return fakeExecResult{err: errors.New("unexpected exec")}
				}
			},
		})
		defer cleanup()

		repoAny, err := New(db, NewPgErrorTranslator())
		if err != nil {
			t.Fatalf("new repo: %v", err)
		}
		repo := repoAny.(*repo)

		if err := repo.CreateRefreshToken(context.Background(), auth.CreateRefreshTokenParams{
			ID:        "refresh-1",
			UserID:    "user-2",
			TokenHash: "hash",
			ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatalf("create refresh token: %v", err)
		}

		if err := repo.CreateRefreshToken(context.Background(), auth.CreateRefreshTokenParams{
			ID:        "refresh-2",
			UserID:    "not-a-uuid",
			TokenHash: "hash",
			ExpiresAt: time.Now().Add(time.Hour),
		}); !errors.Is(err, auth.ErrInvalidUserID) {
			t.Fatalf("expected invalid user id, got %v", err)
		}

		if err := repo.DeactivateRegistrationAuth(context.Background(), "user-2"); err != nil {
			t.Fatalf("deactivate auth: %v", err)
		}
		if err := repo.DeactivateRegistrationAuth(context.Background(), "missing-user"); !errors.Is(err, auth.ErrAuthNotFound) {
			t.Fatalf("expected auth not found, got %v", err)
		}

		if err := repo.DeleteByUserID(context.Background(), "user-2"); err != nil {
			t.Fatalf("delete by user: %v", err)
		}
		if err := repo.DeleteByUserID(context.Background(), "missing-user"); !errors.Is(err, auth.ErrAuthNotFound) {
			t.Fatalf("expected auth not found, got %v", err)
		}
	})

	t.Run("rows affected error branches", func(t *testing.T) {
		db, cleanup := newFakeSQLDB(t, fakeDBConfig{
			exec: func(query string, args []driver.Value) fakeExecResult {
				if strings.Contains(query, "UPDATE auth_credentials") || strings.Contains(query, "DELETE FROM auth_credentials") {
					return fakeExecResult{result: fakeResult{rowsErr: errors.New("rows affected failed")}}
				}
				return fakeExecResult{err: errors.New("unexpected exec")}
			},
		})
		defer cleanup()

		repoAny, err := New(db, NewPgErrorTranslator())
		if err != nil {
			t.Fatalf("new repo: %v", err)
		}
		repo := repoAny.(*repo)

		if err := repo.DeactivateRegistrationAuth(context.Background(), "user-3"); !errors.Is(err, auth.ErrFailedToDeactivateCredential) {
			t.Fatalf("expected deactivate error, got %v", err)
		}
		if err := repo.DeleteByUserID(context.Background(), "user-3"); !errors.Is(err, auth.ErrFailedToDeleteCredential) {
			t.Fatalf("expected delete error, got %v", err)
		}
	})
}

type fakeDBConfig struct {
	queryRow func(query string, args []driver.Value) fakeRowResult
	exec     func(query string, args []driver.Value) fakeExecResult
}

type fakeRowResult struct {
	columns []string
	values  []driver.Value
	err     error
}

type fakeExecResult struct {
	result fakeResult
	err    error
}

type fakeResult struct {
	rows    int64
	rowsErr error
}

func (r fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeResult) RowsAffected() (int64, error) {
	if r.rowsErr != nil {
		return 0, r.rowsErr
	}
	return r.rows, nil
}

var (
	fakeDriverMu sync.Mutex
	fakeDriverFG fakeDBConfig
)

func newFakeSQLDB(t *testing.T, cfg fakeDBConfig) (*sqlx.DB, func()) {
	t.Helper()

	fakeDriverMu.Lock()
	fakeDriverFG = cfg
	fakeDriverMu.Unlock()

	name := fmt.Sprintf("fake-yb-%d", time.Now().UnixNano())
	sql.Register(name, fakeDriver{})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatalf("open fake db: %v", err)
	}

	return sqlx.NewDb(db, "sqlmock"), func() {
		_ = db.Close()
	}
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return fakeConn{}, nil }

type fakeConn struct{}

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("prepare not supported") }
func (fakeConn) Close() error                       { return nil }
func (fakeConn) Begin() (driver.Tx, error)          { return nil, errors.New("tx not supported") }

func (fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	fakeDriverMu.Lock()
	cfg := fakeDriverFG
	fakeDriverMu.Unlock()
	if cfg.queryRow == nil {
		return nil, errors.New("query not configured")
	}
	values := make([]driver.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	res := cfg.queryRow(query, values)
	if res.err != nil {
		return nil, res.err
	}
	return &fakeRows{columns: res.columns, values: res.values}, nil
}

func (fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	fakeDriverMu.Lock()
	cfg := fakeDriverFG
	fakeDriverMu.Unlock()
	if cfg.exec == nil {
		return nil, errors.New("exec not configured")
	}
	values := make([]driver.Value, 0, len(args))
	for _, arg := range args {
		values = append(values, arg.Value)
	}
	res := cfg.exec(query, values)
	if res.err != nil {
		return nil, res.err
	}
	return res.result, nil
}

func (fakeConn) Ping(context.Context) error { return nil }

type fakeRows struct {
	columns []string
	values  []driver.Value
	served  bool
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.served {
		return io.EOF
	}
	r.served = true
	copy(dest, r.values)
	return nil
}
