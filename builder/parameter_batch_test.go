package builder_test

import (
	"reflect"
	"testing"

	"github.com/domainry/domainry-orm/builder"
)

func TestParameterBatchRespectsParameterAndOperationalLimits(t *testing.T) {
	batch := builder.ParameterBatch{MaxParameters: 10, FixedParameters: 1, ParametersPerItem: 2, MaxItems: 3}
	if limit, err := batch.ItemLimit(); err != nil || limit != 3 {
		t.Fatalf("limit=%d err=%v", limit, err)
	}
	want := []builder.BatchRange{{Start: 0, End: 3}, {Start: 3, End: 6}, {Start: 6, End: 7}}
	if got, err := batch.Ranges(7); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ranges=%v err=%v", got, err)
	}
}

func TestParameterBatchRejectsInvalidBudgets(t *testing.T) {
	for _, batch := range []builder.ParameterBatch{
		{},
		{MaxParameters: 2, FixedParameters: 2, ParametersPerItem: 1},
		{MaxParameters: 2, FixedParameters: 0, ParametersPerItem: 0},
	} {
		if _, err := batch.Ranges(1); err == nil {
			t.Fatalf("invalid batch accepted: %#v", batch)
		}
	}
	if ranges, err := (builder.ParameterBatch{MaxParameters: 2, ParametersPerItem: 1}).Ranges(0); err != nil || len(ranges) != 0 {
		t.Fatalf("empty ranges=%v err=%v", ranges, err)
	}
}
