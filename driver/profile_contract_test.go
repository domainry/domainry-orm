package driver_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
	ormdriver "github.com/domainry/domainry-orm/driver"
	"github.com/domainry/domainry-orm/mysql"
	"github.com/domainry/domainry-orm/postgres"
	"github.com/domainry/domainry-orm/sqlite"
)

func TestProfilesOwnEngineSpecificUpsert(t *testing.T) {
	profiles := []ormdriver.Profile{sqlite.NewProfile(), mysql.NewProfile(), postgres.NewProfile()}
	for _, profile := range profiles {
		renderer, err := dialect.ParseRenderer(string(profile.Name()), "", "")
		if err != nil {
			t.Fatal(err)
		}
		insert := builder.NewInsertBuilder(renderer, "jobs").Columns("id", "status").Values("j1", "queued")
		insert, err = profile.ApplyUpsert(insert, []string{"id"}, builder.AssignExpression("status", builder.InsertedValue("status")))
		if err != nil {
			t.Fatalf("%s upsert: %v", profile.Name(), err)
		}
		if _, _, err := insert.Build(); err != nil {
			t.Fatalf("%s build: %v", profile.Name(), err)
		}
	}
}

func TestProfilesOwnLockCapabilitiesAndErrorClassification(t *testing.T) {
	if _, err := sqlite.NewProfile().ApplyClaimLock(nil, true); !errors.Is(err, ormdriver.ErrSkipLockedUnsupported) {
		t.Fatalf("sqlite lock=%v", err)
	}
	if got := postgres.NewProfile().ClassifyError(&pgconn.PgError{Code: "40001"}); got != ormdriver.ErrorSerialization {
		t.Fatalf("postgres classification=%s", got)
	}
	if got := mysql.NewProfile().ClassifyError(errors.New("Deadlock found when trying to get lock")); got != ormdriver.ErrorDeadlock {
		t.Fatalf("mysql classification=%s", got)
	}
	if got := sqlite.NewProfile().ClassifyError(errors.New("database is locked")); got != ormdriver.ErrorUnavailable {
		t.Fatalf("sqlite classification=%s", got)
	}
}
