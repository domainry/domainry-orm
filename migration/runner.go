package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/sqlhost"
)

const DefaultLedgerTable = "_schema_migrations"

const (
	CodeDirty         = "migration.dirty"
	CodeChecksumDrift = "migration.checksum_drift"
	CodeWaitTimeout   = "migration.wait_timeout"
)

// Error reports a stable migration failure while retaining the migration
// identity needed by an application boundary to add product-specific context.
type Error struct {
	Code    string
	Version uint
	Name    string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	identity := fmt.Sprintf("migration %d", e.Version)
	if strings.TrimSpace(e.Name) != "" {
		identity += " (" + strings.TrimSpace(e.Name) + ")"
	}
	if e.Err != nil {
		return identity + ": " + e.Code + ": " + e.Err.Error()
	}
	return identity + ": " + e.Code
}

func (e *Error) Unwrap() error { return e.Err }

// Options controls only persistence mechanics. Migration ownership, names and
// DDL remain in the source module.
type Options struct {
	LedgerTable    string
	WaitTimeout    time.Duration
	PollInterval   time.Duration
	Now            func() time.Time
	InsertConflict func(error) bool
}

// Runner applies source-owned migrations using a portable dirty-ledger
// protocol. Callers that need an advisory lock acquire it before Apply.
type Runner struct {
	database sqlhost.Database
	renderer builder.Renderer
	options  Options
}

func NewRunner(database sqlhost.Database, renderer builder.Renderer, options Options) (*Runner, error) {
	if database == nil || renderer == nil {
		return nil, fmt.Errorf("migration runner requires database and renderer")
	}
	if strings.TrimSpace(options.LedgerTable) == "" {
		options.LedgerTable = DefaultLedgerTable
	}
	if options.WaitTimeout <= 0 {
		options.WaitTimeout = 30 * time.Second
	}
	if options.PollInterval <= 0 {
		options.PollInterval = 25 * time.Millisecond
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Runner{database: database, renderer: renderer, options: options}, nil
}

func (r *Runner) Apply(ctx context.Context, migrations []Migration) error {
	if r == nil || r.database == nil || r.renderer == nil {
		return fmt.Errorf("migration runner is incomplete")
	}
	if err := r.ensureLedger(ctx); err != nil {
		return err
	}
	for _, migration := range migrations {
		if err := r.applyOne(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func Checksum(migration Migration) string {
	payload := fmt.Sprintf("%d\x00%s\x00%s", migration.Version, strings.TrimSpace(migration.Name), strings.Join(migration.Statements, "\x00"))
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (r *Runner) ensureLedger(ctx context.Context) error {
	statement, arguments, err := builder.NewCreateTableBuilder(r.renderer, r.options.LedgerTable).
		IfNotExists().WithoutSystemColumns().Columns(
		builder.DefineColumn("version", builder.BigIntType()).NotNull(),
		builder.DefineColumn("name", builder.TextKeyType(191)).NotNull(),
		builder.DefineColumn("checksum", builder.TextKeyType(64)).NotNull(),
		builder.DefineColumn("dirty", builder.BooleanType()).NotNull(),
		builder.DefineColumn("applied_at", builder.TextKeyType(40)).NotNull(),
	).PrimaryKey("version").Build()
	if err != nil {
		return fmt.Errorf("build migration ledger: %w", err)
	}
	if _, err := r.database.ExecContext(ctx, statement, arguments...); err != nil {
		return fmt.Errorf("prepare migration ledger: %w", err)
	}
	return nil
}

func (r *Runner) applyOne(ctx context.Context, migration Migration) error {
	checksum := Checksum(migration)
	query, arguments, err := builder.NewSelectBuilder(r.renderer, r.options.LedgerTable).
		Columns("checksum", "dirty").Where(builder.Equal("version", migration.Version)).Build()
	if err != nil {
		return err
	}
	applied, dirty, found, err := readLedger(ctx, r.database, query, arguments)
	if err != nil {
		return fmt.Errorf("inspect migration %d: %w", migration.Version, err)
	}
	if found {
		return validateLedger(migration, checksum, applied, dirty)
	}
	insert, insertArguments, err := builder.NewInsertBuilder(r.renderer, r.options.LedgerTable).
		Columns("version", "name", "checksum", "dirty", "applied_at").
		Values(migration.Version, strings.TrimSpace(migration.Name), checksum, true, "").Build()
	if err != nil {
		return err
	}
	if _, err := r.database.ExecContext(ctx, insert, insertArguments...); err != nil {
		if r.options.InsertConflict == nil || !r.options.InsertConflict(err) {
			return fmt.Errorf("record dirty migration %d: %w", migration.Version, err)
		}
		return r.waitForPeer(ctx, migration, checksum, query, arguments)
	}
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.Version, err)
	}
	defer tx.Rollback()
	for _, statement := range migration.Statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %d: %w", migration.Version, err)
		}
	}
	complete, completeArguments, err := builder.NewUpdateBuilder(r.renderer, r.options.LedgerTable).
		Set("dirty", false).Set("applied_at", r.options.Now().UTC().Format(time.RFC3339Nano)).
		Where(builder.And(builder.Equal("version", migration.Version), builder.Equal("checksum", checksum))).Build()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, complete, completeArguments...); err != nil {
		return fmt.Errorf("complete migration %d ledger: %w", migration.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.Version, err)
	}
	return nil
}

func (r *Runner) waitForPeer(ctx context.Context, migration Migration, checksum, query string, arguments []any) error {
	deadline := time.NewTimer(r.options.WaitTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(r.options.PollInterval)
	defer ticker.Stop()
	for {
		applied, dirty, found, err := readLedger(ctx, r.database, query, arguments)
		if err != nil {
			return fmt.Errorf("inspect concurrent migration %d: %w", migration.Version, err)
		}
		if found && applied != checksum {
			return &Error{Code: CodeChecksumDrift, Version: migration.Version, Name: migration.Name}
		}
		if found && !dirty {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return &Error{Code: CodeWaitTimeout, Version: migration.Version, Name: migration.Name}
		case <-ticker.C:
		}
	}
}

func readLedger(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, query string, arguments []any) (string, bool, bool, error) {
	var checksum string
	var dirty bool
	err := queryer.QueryRowContext(ctx, query, arguments...).Scan(&checksum, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, false, nil
	}
	return checksum, dirty, err == nil, err
}

func validateLedger(migration Migration, expected, applied string, dirty bool) error {
	if dirty {
		return &Error{Code: CodeDirty, Version: migration.Version, Name: migration.Name}
	}
	if applied != expected {
		return &Error{Code: CodeChecksumDrift, Version: migration.Version, Name: migration.Name}
	}
	return nil
}
