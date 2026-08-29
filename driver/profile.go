// Package driver defines database-engine strategies consumed by persistence
// adapters. Repositories depend on Profile instead of branching on dialect
// names themselves.
package driver

import (
	"errors"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
)

type ErrorKind string

const (
	ErrorUnknown             ErrorKind = "unknown"
	ErrorConflict            ErrorKind = "conflict"
	ErrorConstraintViolation ErrorKind = "constraint_violation"
	ErrorSerialization       ErrorKind = "serialization_failure"
	ErrorDeadlock            ErrorKind = "deadlock"
	ErrorUnavailable         ErrorKind = "unavailable"
	ErrorTimeout             ErrorKind = "timeout"
)

type Capabilities struct {
	Returning             bool
	RowLock               bool
	SkipLocked            bool
	AdvisoryLock          bool
	PartialIndex          bool
	IndexIfNotExists      bool
	RowLevelSecurity      bool
	NativeJSON            bool
	TransactionalDDL      bool
	MaximumBindParameters int
}

type Profile interface {
	Name() dialect.Name
	Capabilities() Capabilities
	ApplyUpsert(*builder.InsertBuilder, []string, ...builder.Assignment) (*builder.InsertBuilder, error)
	ApplyReturning(*builder.InsertBuilder, ...string) (*builder.InsertBuilder, error)
	ApplyClaimLock(*builder.SelectBuilder, bool) (*builder.SelectBuilder, error)
	ClassifyError(error) ErrorKind
}

var (
	ErrReturningUnsupported  = errors.New("SQL profile does not support RETURNING")
	ErrRowLockUnsupported    = errors.New("SQL profile does not support row locks")
	ErrSkipLockedUnsupported = errors.New("SQL profile does not support SKIP LOCKED")
)
