package sqlhost

import (
	"context"
	"database/sql"
)

type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// DBTX is the SQL surface shared by *sql.DB and *sql.Tx. It deliberately
// excludes commit, rollback, close, and connection-pool lifecycle methods.
type DBTX interface {
	Executor
	Queryer
}

type Beginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

type Database interface {
	DBTX
	Beginner
}

var _ Database = (*sql.DB)(nil)
var _ DBTX = (*sql.Tx)(nil)
