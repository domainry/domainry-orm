package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
)

// Connection is the session-bound SQL surface required by advisory locks.
// Acquire and Release must operate on the same physical connection.
type Connection interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Release func(context.Context) error

type Locker interface {
	Acquire(context.Context, Connection, string) (Release, error)
}

// LockKey returns a stable, bounded key while keeping product ownership in the
// caller-provided namespace.
func LockKey(namespace string) string {
	return NamespacedLockKey("migration", namespace)
}

// NamespacedLockKey preserves a caller-owned, human-readable prefix while
// bounding the untrusted namespace portion.
func NamespacedLockKey(prefix, namespace string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(namespace)))
	return strings.TrimSpace(prefix) + "-" + hex.EncodeToString(sum[:16])
}
