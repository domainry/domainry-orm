package sqlite

import (
	"context"
	"errors"
	"strings"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
	ormdriver "github.com/domainry/domainry-orm/driver"
)

type Profile struct{}

func NewProfile() Profile          { return Profile{} }
func (Profile) Name() dialect.Name { return dialect.SQLite }
func (Profile) Capabilities() ormdriver.Capabilities {
	return ormdriver.Capabilities{Returning: true, PartialIndex: true, IndexIfNotExists: true, TransactionalDDL: true, MaximumBindParameters: 999}
}
func (Profile) ApplyUpsert(value *builder.InsertBuilder, keys []string, assignments ...builder.Assignment) (*builder.InsertBuilder, error) {
	return value.OnConflictDoUpdate(keys, assignments...), nil
}
func (Profile) ApplyReturning(value *builder.InsertBuilder, columns ...string) (*builder.InsertBuilder, error) {
	return value.Returning(columns...), nil
}
func (Profile) ApplyClaimLock(*builder.SelectBuilder, bool) (*builder.SelectBuilder, error) {
	return nil, ormdriver.ErrRowLockUnsupported
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
