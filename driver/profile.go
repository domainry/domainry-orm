package driver

import (
	"context"
	"database/sql"
	"errors"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
	"github.com/domainry/domainry-orm/schema"
	"github.com/domainry/domainry-orm/sqlhost"
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
	TextKeyColumnType(int) string
	ApplyUpsert(*query.InsertBuilder, []string, ...query.Assignment) (*query.InsertBuilder, error)
	ApplyReturning(*query.InsertBuilder, ...string) (*query.InsertBuilder, error)
	ApplyClaimLock(*query.SelectBuilder, bool) (*query.SelectBuilder, error)
	ApplyCreateIndex(*schema.IndexBuilder) *schema.IndexBuilder
	IsCreateIndexAlreadyExists(error) bool
	ClassifyError(error) ErrorKind
	BeginWrite(context.Context, *sql.DB) (Transaction, error)
}

type Transaction interface {
	sqlhost.DBTX
	Commit(context.Context) error
	Rollback(context.Context) error
}

type StandardTransaction struct{ *sql.Tx }

func BeginSerializable(ctx context.Context, database *sql.DB) (Transaction, error) {
	value, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	return StandardTransaction{Tx: value}, nil
}

func (t StandardTransaction) Commit(context.Context) error   { return t.Tx.Commit() }
func (t StandardTransaction) Rollback(context.Context) error { return t.Tx.Rollback() }

var (
	ErrReturningUnsupported  = errors.New("SQL profile does not support RETURNING")
	ErrRowLockUnsupported    = errors.New("SQL profile does not support row locks")
	ErrSkipLockedUnsupported = errors.New("SQL profile does not support SKIP LOCKED")
)
