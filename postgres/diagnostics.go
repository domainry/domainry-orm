package postgres

import (
	"context"
	"crypto/x509"
	"database/sql"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type CapabilityError struct{ Kind string }

func (e CapabilityError) Error() string {
	return "PostgreSQL capability check failed (" + e.Kind + ")"
}

const (
	FailureDNS                = "dns"
	FailureNetworkIPv4        = "network_ipv4"
	FailureNetworkIPv6        = "network_ipv6"
	FailureTLS                = "tls"
	FailureAuthentication     = "authentication"
	FailurePoolExhausted      = "pool_exhausted"
	FailureServerUnavailable  = "server_unavailable"
	FailureSchemaIncompatible = "schema_incompatible"
	FailureUnknown            = "unknown"
)

type Capabilities struct {
	ServerVersion string
	Database      string
	User          string
	CurrentSchema string
	TLS           bool
	ReadOnly      bool
	InRecovery    bool
	SchemaExists  bool
	SchemaUsage   bool
	SchemaCreate  bool
	AdvisoryLocks bool
}

func Probe(ctx context.Context, db *sql.DB, schema string) (Capabilities, error) {
	if db == nil {
		return Capabilities{}, errors.New("PostgreSQL database is required")
	}
	const query = `SELECT
current_setting('server_version'),
current_database(),
current_user,
COALESCE(current_schema(), ''),
EXISTS (SELECT 1 FROM pg_stat_ssl WHERE pid = pg_backend_pid() AND ssl),
current_setting('default_transaction_read_only') = 'on',
pg_is_in_recovery(),
EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1),
COALESCE(has_schema_privilege(current_user, (SELECT oid FROM pg_namespace WHERE nspname = $1), 'USAGE'), false),
COALESCE(has_schema_privilege(current_user, (SELECT oid FROM pg_namespace WHERE nspname = $1), 'CREATE'), false),
to_regprocedure('pg_try_advisory_xact_lock(bigint)') IS NOT NULL`
	var capability Capabilities
	err := db.QueryRowContext(ctx, query, schema).Scan(
		&capability.ServerVersion,
		&capability.Database,
		&capability.User,
		&capability.CurrentSchema,
		&capability.TLS,
		&capability.ReadOnly,
		&capability.InRecovery,
		&capability.SchemaExists,
		&capability.SchemaUsage,
		&capability.SchemaCreate,
		&capability.AdvisoryLocks,
	)
	if err != nil {
		return Capabilities{}, err
	}
	return capability, nil
}

func ProbeWithBackoff(ctx context.Context, db *sql.DB, schema string) (Capabilities, error) {
	return RetryPoolProbe(ctx, 3, 100*time.Millisecond, func(ctx context.Context) (Capabilities, error) {
		return Probe(ctx, db, schema)
	}, WaitForPoolRetry)
}

func RetryPoolProbe(ctx context.Context, attempts int, initial time.Duration, probe func(context.Context) (Capabilities, error), wait func(context.Context, time.Duration) error) (Capabilities, error) {
	if attempts < 1 {
		attempts = 1
	}
	if initial <= 0 {
		initial = 100 * time.Millisecond
	}
	for attempt := 1; ; attempt++ {
		capability, err := probe(ctx)
		if err == nil {
			return capability, nil
		}
		if ClassifyConnectionFailure(err) != FailurePoolExhausted || attempt == attempts {
			return Capabilities{}, err
		}
		delay := initial << (attempt - 1)
		if delay > time.Second {
			delay = time.Second
		}
		if err := wait(ctx, delay); err != nil {
			return Capabilities{}, err
		}
	}
}

func WaitForPoolRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ValidateCapabilities(requireTLS, migrationConfigured bool, query, migrator Capabilities) error {
	if query.Database == "" {
		return CapabilityError{Kind: FailureSchemaIncompatible}
	}
	if query.ReadOnly || query.InRecovery {
		return CapabilityError{Kind: "read_only"}
	}
	if requireTLS && !query.TLS {
		return CapabilityError{Kind: FailureTLS}
	}
	if migrationConfigured {
		if migrator.Database == "" || query.Database != migrator.Database {
			return CapabilityError{Kind: FailureSchemaIncompatible}
		}
		if migrator.ReadOnly || migrator.InRecovery {
			return CapabilityError{Kind: "read_only"}
		}
		if requireTLS && !migrator.TLS {
			return CapabilityError{Kind: FailureTLS}
		}
	}
	return nil
}

func ClassifyConnectionFailure(err error) string {
	if err == nil {
		return ""
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return FailureDNS
	}
	var certificateError x509.UnknownAuthorityError
	if errors.As(err, &certificateError) {
		return FailureTLS
	}
	var networkError *net.OpError
	if errors.As(err, &networkError) {
		if address, ok := networkError.Addr.(*net.TCPAddr); ok && address.IP != nil {
			if address.IP.To4() == nil {
				return FailureNetworkIPv6
			}
			return FailureNetworkIPv4
		}
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "28P01", "28000":
			return FailureAuthentication
		case "53300", "53400":
			return FailurePoolExhausted
		case "3D000", "3F000", "42P01":
			return FailureSchemaIncompatible
		case "57P01", "57P02", "57P03":
			return FailureServerUnavailable
		}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "certificate"), strings.Contains(message, "tls"), strings.Contains(message, "ssl"):
		return FailureTLS
	case strings.Contains(message, "network is unreachable"), strings.Contains(message, "no route to host"):
		return FailureNetworkIPv4
	case strings.Contains(message, "server is unavailable"), strings.Contains(message, "database is unavailable"):
		return FailureServerUnavailable
	case strings.Contains(message, "too many connections"), strings.Contains(message, "max client connections"):
		return FailurePoolExhausted
	default:
		return FailureUnknown
	}
}
