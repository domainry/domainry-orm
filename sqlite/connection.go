package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultBusyTimeout        = 5 * time.Second
	DefaultMaxOpenConnections = 8
	JournalModeWAL            = "WAL"
)

var ErrOwnedConnectionRequired = errors.New("sqlite owned connection initialization required")

// OwnedConnectionConfig describes initialization performed exactly once by
// the process that owns a SQLite pool. Embedded modules must consume the
// host-provided pool and must not call InitializeOwned again.
type OwnedConnectionConfig struct {
	DataSource         string        `json:"data_source" yaml:"data_source"`
	BusyTimeout        time.Duration `json:"busy_timeout" yaml:"busy_timeout"`
	MaxOpenConnections int           `json:"max_open_connections" yaml:"max_open_connections"`
	MaxIdleConnections int           `json:"max_idle_connections" yaml:"max_idle_connections"`
	ForeignKeys        bool          `json:"foreign_keys" yaml:"foreign_keys"`
	JournalMode        string        `json:"journal_mode" yaml:"journal_mode"`
}

func DefaultOwnedConnectionConfig(dataSource string) OwnedConnectionConfig {
	return OwnedConnectionConfig{
		DataSource: strings.TrimSpace(dataSource), BusyTimeout: DefaultBusyTimeout,
		MaxOpenConnections: DefaultMaxOpenConnections, MaxIdleConnections: DefaultMaxOpenConnections,
		ForeignKeys: true, JournalMode: JournalModeWAL,
	}
}

func (config OwnedConnectionConfig) Validate() error {
	if strings.TrimSpace(config.DataSource) == "" {
		return fmt.Errorf("sqlite data source is required")
	}
	if strings.Contains(strings.ToLower(config.DataSource), "_pragma=") {
		return fmt.Errorf("sqlite data source must not override ORM-managed PRAGMAs")
	}
	if config.BusyTimeout <= 0 {
		return fmt.Errorf("sqlite busy timeout must be positive")
	}
	if config.MaxOpenConnections < 1 {
		return fmt.Errorf("sqlite max open connections must be positive")
	}
	if config.MaxIdleConnections < 0 || config.MaxIdleConnections > config.MaxOpenConnections {
		return fmt.Errorf("sqlite max idle connections must be between zero and max open connections")
	}
	if mode := strings.ToUpper(strings.TrimSpace(config.JournalMode)); mode != JournalModeWAL {
		return fmt.Errorf("sqlite journal mode must be WAL")
	}
	return nil
}

// DSN returns a modernc-compatible data source whose connection-local PRAGMAs
// are applied whenever database/sql opens a new physical connection.
func (config OwnedConnectionConfig) DSN() (string, error) {
	if err := config.Validate(); err != nil {
		return "", err
	}
	dsn := strings.TrimSpace(config.DataSource)
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	milliseconds := config.BusyTimeout.Milliseconds()
	foreignKeys := 0
	if config.ForeignKeys {
		foreignKeys = 1
	}
	return fmt.Sprintf("%s%s_pragma=busy_timeout%%28%d%%29&_pragma=foreign_keys%%28%d%%29", dsn, separator, milliseconds, foreignKeys), nil
}

// InitializeOwned configures a host-owned pool. It is intentionally not part
// of Profile: query profiles are safe for modules, while pool initialization
// is an ownership boundary that only a standalone host may cross.
func InitializeOwned(ctx context.Context, database *sql.DB, config OwnedConnectionConfig) error {
	if database == nil {
		return ErrOwnedConnectionRequired
	}
	if err := config.Validate(); err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(config.DataSource)), "file:") {
		if err := os.MkdirAll(filepath.Dir(config.DataSource), 0o755); err != nil {
			return fmt.Errorf("create sqlite database directory: %w", err)
		}
	}
	database.SetMaxOpenConns(config.MaxOpenConnections)
	database.SetMaxIdleConns(config.MaxIdleConnections)
	milliseconds := config.BusyTimeout.Milliseconds()
	if _, err := database.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", milliseconds)); err != nil {
		return fmt.Errorf("configure sqlite busy timeout: %w", err)
	}
	foreignKeys := "OFF"
	if config.ForeignKeys {
		foreignKeys = "ON"
	}
	if _, err := database.ExecContext(ctx, "PRAGMA foreign_keys = "+foreignKeys); err != nil {
		return fmt.Errorf("configure sqlite foreign keys: %w", err)
	}
	if err := configureWAL(ctx, database, config.BusyTimeout); err != nil {
		return fmt.Errorf("configure sqlite WAL: %w", err)
	}
	return nil
}

func configureWAL(ctx context.Context, database *sql.DB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		_, err := database.ExecContext(ctx, "PRAGMA journal_mode = "+JournalModeWAL)
		if err == nil || !sqliteBusy(err) {
			return err
		}
		if time.Now().Add(10 * time.Millisecond).After(deadline) {
			return err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func sqliteBusy(err error) bool {
	for current := err; current != nil; current = errors.Unwrap(current) {
		if coded, ok := current.(interface{ Code() int }); ok && coded.Code()&0xff == 5 {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_busy") || strings.Contains(message, "database is locked")
}
