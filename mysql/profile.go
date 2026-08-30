package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
	ormdriver "github.com/domainry/domainry-orm/driver"
	"github.com/domainry/domainry-orm/query"
	"github.com/domainry/domainry-orm/schema"
)

type Profile struct{}

func NewProfile() Profile                           { return Profile{} }
func (Profile) Name() dialect.Name                  { return dialect.MySQL }
func (Profile) TextKeyColumnType(length int) string { return fmt.Sprintf("VARCHAR(%d)", length) }
func (Profile) Capabilities() ormdriver.Capabilities {
	return ormdriver.Capabilities{RowLock: true, SkipLocked: true, AdvisoryLock: true, NativeJSON: true, MaximumBindParameters: 65535}
}
func (Profile) ApplyUpsert(value *query.InsertBuilder, _ []string, assignments ...query.Assignment) (*query.InsertBuilder, error) {
	return value.OnDuplicateKeyUpdate(assignments...), nil
}
func (Profile) ApplyReturning(*query.InsertBuilder, ...string) (*query.InsertBuilder, error) {
	return nil, ormdriver.ErrReturningUnsupported
}
func (Profile) ApplyClaimLock(value *query.SelectBuilder, skipLocked bool) (*query.SelectBuilder, error) {
	if skipLocked {
		return value.ForUpdate("SKIP LOCKED"), nil
	}
	return value.ForUpdate(), nil
}
func (Profile) ApplyCreateIndex(value *schema.IndexBuilder) *schema.IndexBuilder {
	return value
}
func (Profile) IsCreateIndexAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key name") || strings.Contains(message, "error 1061")
}
func (Profile) ClassifyError(err error) ormdriver.ErrorKind {
	if err == nil {
		return ormdriver.ErrorUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ormdriver.ErrorTimeout
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "deadlock"):
		return ormdriver.ErrorDeadlock
	case strings.Contains(message, "lock wait timeout"):
		return ormdriver.ErrorTimeout
	case strings.Contains(message, "duplicate entry"):
		return ormdriver.ErrorConflict
	case strings.Contains(message, "constraint"):
		return ormdriver.ErrorConstraintViolation
	case strings.Contains(message, "connection"), strings.Contains(message, "server has gone away"):
		return ormdriver.ErrorUnavailable
	default:
		return ormdriver.ErrorUnknown
	}
}
func (Profile) BeginWrite(ctx context.Context, database *sql.DB) (ormdriver.Transaction, error) {
	return ormdriver.BeginSerializable(ctx, database)
}
