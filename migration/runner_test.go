package migration

import (
	"testing"
)

func TestChecksumIsStableAndSensitiveToIdentity(t *testing.T) {
	base := Migration{Version: 1, Name: " foundation ", Statements: []string{"CREATE TABLE example(id TEXT)"}}
	if Checksum(base) != Checksum(Migration{Version: 1, Name: "foundation", Statements: base.Statements}) {
		t.Fatal("checksum must normalize migration name")
	}
	if Checksum(base) == Checksum(Migration{Version: 2, Name: "foundation", Statements: base.Statements}) {
		t.Fatal("checksum must include version")
	}
	if Checksum(base) == Checksum(Migration{Version: 1, Name: "foundation", Statements: []string{"CREATE TABLE other(id TEXT)"}}) {
		t.Fatal("checksum must include statements")
	}
}

func TestValidateLedgerReturnsStableErrors(t *testing.T) {
	migration := Migration{Version: 7, Name: "example"}
	if err := validateLedger(migration, "same", "same", false); err != nil {
		t.Fatalf("valid ledger: %v", err)
	}
	assertCode := func(err error, code string) {
		t.Helper()
		migrationErr, ok := err.(*Error)
		if !ok || migrationErr.Code != code {
			t.Fatalf("error=%#v want code %q", err, code)
		}
	}
	assertCode(validateLedger(migration, "same", "same", true), CodeDirty)
	assertCode(validateLedger(migration, "expected", "different", false), CodeChecksumDrift)
}

func TestLockKeyIsStableAndNamespaced(t *testing.T) {
	if LockKey(" party/schema ") != LockKey("party/schema") {
		t.Fatal("lock key must normalize surrounding whitespace")
	}
	if LockKey("party/schema") == LockKey("notification/schema") {
		t.Fatal("lock key must preserve namespace ownership")
	}
	if got := NamespacedLockKey("party-migration", "schema"); got[:16] != "party-migration-" {
		t.Fatalf("namespaced lock key=%q", got)
	}
}
