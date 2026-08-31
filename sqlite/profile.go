package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/domainry/domainry-orm/dialect"
	ormdriver "github.com/domainry/domainry-orm/driver"
	"github.com/domainry/domainry-orm/query"
	"github.com/domainry/domainry-orm/schema"
)

type Profile struct{}

func NewProfile() Profile                    { return Profile{} }
func (Profile) Name() dialect.Name           { return dialect.SQLite }
func (Profile) TextKeyColumnType(int) string { return "TEXT" }
func (Profile) Capabilities() ormdriver.Capabilities {
	return ormdriver.Capabilities{Returning: true, PartialIndex: true, IndexIfNotExists: true, TransactionalDDL: true, MaximumBindParameters: 999}
}
func (Profile) ApplyUpsert(value *query.InsertBuilder, keys []string, assignments ...query.Assignment) (*query.InsertBuilder, error) {
	return value.OnConflictDoUpdate(keys, assignments...), nil
}
func (Profile) ApplyReturning(value *query.InsertBuilder, columns ...string) (*query.InsertBuilder, error) {
	return value.Returning(columns...), nil
}
func (Profile) ApplyClaimLock(value *query.SelectBuilder, skipLocked bool) (*query.SelectBuilder, error) {
	if skipLocked {
		return nil, ormdriver.ErrSkipLockedUnsupported
	}
	return value, nil
}
func (Profile) ApplyCreateIndex(value *schema.IndexBuilder) *schema.IndexBuilder {
	return value.IfNotExists()
}
func (Profile) IsCreateIndexAlreadyExists(error) bool { return false }
func (Profile) ClassifyError(err error) ormdriver.ErrorKind {
	if err == nil {
		return ormdriver.ErrorUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ormdriver.ErrorTimeout
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "locked"), strings.Contains(message, "busy"):
		return ormdriver.ErrorUnavailable
	case strings.Contains(message, "unique constraint"):
		return ormdriver.ErrorConflict
	case strings.Contains(message, "constraint"):
		return ormdriver.ErrorConstraintViolation
	default:
		return ormdriver.ErrorUnknown
	}
}

type immediateTransaction struct{ connection *sql.Conn }

func (Profile) BeginWrite(ctx context.Context, database *sql.DB) (ormdriver.Transaction, error) {
	connection, err := database.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return &immediateTransaction{connection: connection}, nil
}
func (t *immediateTransaction) ExecContext(ctx context.Context, queryValue string, args ...any) (sql.Result, error) {
	return t.connection.ExecContext(ctx, queryValue, args...)
}
func (t *immediateTransaction) QueryContext(ctx context.Context, queryValue string, args ...any) (*sql.Rows, error) {
	return t.connection.QueryContext(ctx, queryValue, args...)
}
func (t *immediateTransaction) QueryRowContext(ctx context.Context, queryValue string, args ...any) *sql.Row {
	return t.connection.QueryRowContext(ctx, queryValue, args...)
}
func (t *immediateTransaction) Commit(ctx context.Context) error {
	_, err := t.connection.ExecContext(ctx, "COMMIT")
	closeErr := t.connection.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func (t *immediateTransaction) Rollback(ctx context.Context) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := t.connection.ExecContext(cleanupCtx, "ROLLBACK")
	closeErr := t.connection.Close()
	if err != nil {
		return err
	}
	return closeErr
}
