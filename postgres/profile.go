package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
	ormdriver "github.com/domainry/domainry-orm/driver"
)

type Profile struct{}

func NewProfile() Profile          { return Profile{} }
func (Profile) Name() dialect.Name { return dialect.Postgres }
func (Profile) Capabilities() ormdriver.Capabilities {
	return ormdriver.Capabilities{Returning: true, RowLock: true, SkipLocked: true, AdvisoryLock: true, PartialIndex: true, IndexIfNotExists: true, RowLevelSecurity: true, NativeJSON: true, TransactionalDDL: true, MaximumBindParameters: 65535}
}
func (Profile) ApplyUpsert(value *builder.InsertBuilder, keys []string, assignments ...builder.Assignment) (*builder.InsertBuilder, error) {
	return value.OnConflictDoUpdate(keys, assignments...), nil
}
func (Profile) ApplyReturning(value *builder.InsertBuilder, columns ...string) (*builder.InsertBuilder, error) {
	return value.Returning(columns...), nil
}
func (Profile) ApplyClaimLock(value *builder.SelectBuilder, skipLocked bool) (*builder.SelectBuilder, error) {
	if skipLocked {
		return value.ForUpdate("SKIP LOCKED"), nil
	}
	return value.ForUpdate(), nil
}
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
