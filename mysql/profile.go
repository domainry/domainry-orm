package mysql

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
func (Profile) Name() dialect.Name { return dialect.MySQL }
func (Profile) Capabilities() ormdriver.Capabilities {
	return ormdriver.Capabilities{RowLock: true, SkipLocked: true, AdvisoryLock: true, NativeJSON: true, MaximumBindParameters: 65535}
}
func (Profile) ApplyUpsert(value *builder.InsertBuilder, _ []string, assignments ...builder.Assignment) (*builder.InsertBuilder, error) {
	return value.OnDuplicateKeyUpdate(assignments...), nil
}
func (Profile) ApplyReturning(*builder.InsertBuilder, ...string) (*builder.InsertBuilder, error) {
	return nil, ormdriver.ErrReturningUnsupported
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
