package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOwnedConnectionConfigBuildsPerConnectionPragmas(t *testing.T) {
	config := DefaultOwnedConnectionConfig("runtime.db")
	dsn, err := config.DSN()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"runtime.db?", "_pragma=busy_timeout%285000%29", "_pragma=foreign_keys%281%29"} {
		if !strings.Contains(dsn, fragment) {
			t.Fatalf("dsn %q missing %q", dsn, fragment)
		}
	}
	config.DataSource = "file:runtime.db?cache=shared"
	if dsn, err = config.DSN(); err != nil || !strings.Contains(dsn, "cache=shared&_pragma=") {
		t.Fatalf("query DSN=%q err=%v", dsn, err)
	}
}

func TestOwnedConnectionConfigRejectsUnsafeValues(t *testing.T) {
	valid := DefaultOwnedConnectionConfig("runtime.db")
	tests := []OwnedConnectionConfig{
		{},
		func() OwnedConnectionConfig {
			value := valid
			value.DataSource = "runtime.db?_pragma=busy_timeout(1)"
			return value
		}(),
		func() OwnedConnectionConfig { value := valid; value.BusyTimeout = 0; return value }(),
		func() OwnedConnectionConfig { value := valid; value.MaxOpenConnections = 0; return value }(),
		func() OwnedConnectionConfig { value := valid; value.MaxIdleConnections = 9; return value }(),
		func() OwnedConnectionConfig { value := valid; value.JournalMode = "DELETE"; return value }(),
	}
	for _, config := range tests {
		if err := config.Validate(); err == nil {
			t.Fatalf("config %#v unexpectedly valid", config)
		}
	}
}

func TestInitializeOwnedConfiguresPoolAndPropagatesContext(t *testing.T) {
	state := &connectionState{}
	database := sql.OpenDB(connectionConnector{state: state})
	defer database.Close()
	config := DefaultOwnedConnectionConfig("runtime.db")
	config.MaxOpenConnections, config.MaxIdleConnections = 6, 3
	if err := InitializeOwned(t.Context(), database, config); err != nil {
		t.Fatal(err)
	}
	if stats := database.Stats(); stats.MaxOpenConnections != 6 {
		t.Fatalf("max open=%d", stats.MaxOpenConnections)
	}
	if len(state.statements) != 3 || !strings.Contains(state.statements[0], "busy_timeout") || !strings.Contains(state.statements[2], "journal_mode = WAL") {
		t.Fatalf("statements=%#v", state.statements)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := InitializeOwned(ctx, database, config); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled initialization error=%v", err)
	}
	if err := InitializeOwned(t.Context(), nil, config); !errors.Is(err, ErrOwnedConnectionRequired) {
		t.Fatalf("nil database error=%v", err)
	}
}

type connectionState struct{ statements []string }
type connectionConnector struct{ state *connectionState }

func (connector connectionConnector) Connect(context.Context) (driver.Conn, error) {
	return &connectionConn{state: connector.state}, nil
}
func (connectionConnector) Driver() driver.Driver { return connectionDriver{} }

type connectionDriver struct{}

func (connectionDriver) Open(string) (driver.Conn, error) { return nil, errors.New("unsupported") }

type connectionConn struct{ state *connectionState }

func (*connectionConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*connectionConn) Close() error                        { return nil }
func (*connectionConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }
func (connection *connectionConn) ExecContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connection.state.statements = append(connection.state.statements, query)
	return driver.RowsAffected(0), nil
}

var _ driver.ExecerContext = (*connectionConn)(nil)

func TestDefaultBusyTimeoutRemainsBounded(t *testing.T) {
	if DefaultBusyTimeout != 5*time.Second {
		t.Fatalf("busy timeout=%s", DefaultBusyTimeout)
	}
}
