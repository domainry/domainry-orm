package batch_test

import (
	"reflect"
	"testing"

	"github.com/domainry/domainry-orm/batch"
)

func TestParametersRespectsEngineAndOperationalLimits(t *testing.T) {
	ranges, err := (batch.Parameters{Max: 10, Fixed: 1, PerItem: 2, MaxItems: 3}).Ranges(8)
	if err != nil {
		t.Fatal(err)
	}
	want := []batch.Range{{Start: 0, End: 3}, {Start: 3, End: 6}, {Start: 6, End: 8}}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("unexpected ranges: got %#v want %#v", ranges, want)
	}
}

func TestParametersRejectsInvalidBudget(t *testing.T) {
	if _, err := (batch.Parameters{Max: 2, Fixed: 2, PerItem: 1}).ItemLimit(); err == nil {
		t.Fatal("expected invalid budget error")
	}
}
