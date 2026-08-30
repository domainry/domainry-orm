package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/domainry/domainry-orm/dialect"
	ormdriver "github.com/domainry/domainry-orm/driver"
	"github.com/domainry/domainry-orm/query"
	"github.com/domainry/domainry-orm/schema"
)

type Profile struct{}

func NewProfile() Profile                    { return Profile{} }
func (Profile) Name() dialect.Name           { return dialect.Postgres }
func (Profile) TextKeyColumnType(int) string { return "TEXT" }
func (Profile) Capabilities() ormdriver.Capabilities {
	return ormdriver.Capabilities{Returning: true, RowLock: true, SkipLocked: true, AdvisoryLock: true, PartialIndex: true, IndexIfNotExists: true, RowLevelSecurity: true, NativeJSON: true, TransactionalDDL: true, MaximumBindParameters: 65535}
}
func (Profile) ApplyUpsert(value *query.InsertBuilder, keys []string, assignments ...query.Assignment) (*query.InsertBuilder, error) {
	return value.OnConflictDoUpdate(keys, assignments...), nil
}
func (Profile) ApplyReturning(value *query.InsertBuilder, columns ...string) (*query.InsertBuilder, error) {
	return value.Returning(columns...), nil
}
func (Profile) ApplyClaimLock(value *query.SelectBuilder, skipLocked bool) (*query.SelectBuilder, error) {
	if skipLocked {
		return value.ForUpdate("SKIP LOCKED"), nil
	}
	return value.ForUpdate(), nil
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
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return ormdriver.ErrorUnknown
	}
	switch postgresError.Code {
	case "23505":
		return ormdriver.ErrorConflict
	case "23502", "23503", "23514", "23P01":
		return ormdriver.ErrorConstraintViolation
	case "40001":
		return ormdriver.ErrorSerialization
	case "40P01":
		return ormdriver.ErrorDeadlock
	case "53300", "57P01", "57P02", "57P03":
		return ormdriver.ErrorUnavailable
	case "57014":
		return ormdriver.ErrorTimeout
	default:
		return ormdriver.ErrorUnknown
	}
}
func (Profile) BeginWrite(ctx context.Context, database *sql.DB) (ormdriver.Transaction, error) {
	return ormdriver.BeginSerializable(ctx, database)
}
