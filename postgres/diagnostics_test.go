package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRetryPoolProbe(t *testing.T) {
	calls := 0
	waits := []time.Duration{}
	got, err := RetryPoolProbe(t.Context(), 3, 100*time.Millisecond, func(context.Context) (Capabilities, error) {
		calls++
		if calls < 3 {
			return Capabilities{}, &pgconn.PgError{Code: "53300"}
		}
		return Capabilities{Database: "runtime"}, nil
	}, func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	})
	if err != nil || got.Database != "runtime" || calls != 3 || !reflect.DeepEqual(waits, []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}) {
		t.Fatalf("capabilities=%#v calls=%d waits=%v err=%v", got, calls, waits, err)
	}
}

func TestValidateCapabilities(t *testing.T) {
	query := Capabilities{Database: "runtime", TLS: true}
	if err := ValidateCapabilities(true, true, query, query); err != nil {
		t.Fatal(err)
	}
	var capabilityError CapabilityError
	err := ValidateCapabilities(false, false, Capabilities{}, Capabilities{})
	if !errors.As(err, &capabilityError) || capabilityError.Kind != FailureSchemaIncompatible {
		t.Fatalf("error = %v", err)
	}
}

func TestClassifyConnectionFailure(t *testing.T) {
	if got := ClassifyConnectionFailure(&pgconn.PgError{Code: "28P01"}); got != FailureAuthentication {
		t.Fatalf("classification = %q", got)
	}
	if got := ClassifyConnectionFailure(errors.New("too many connections")); got != FailurePoolExhausted {
		t.Fatalf("classification = %q", got)
	}
}
