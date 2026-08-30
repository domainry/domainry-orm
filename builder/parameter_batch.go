package builder

import "github.com/domainry/domainry-orm/batch"

// Deprecated: use batch.Range.
type BatchRange = batch.Range

// ParameterBatch is the source-compatible legacy shape for batch.Parameters.
// Deprecated: use batch.Parameters.
type ParameterBatch struct {
	MaxParameters     int
	FixedParameters   int
	ParametersPerItem int
	MaxItems          int
}

func (b ParameterBatch) parameters() batch.Parameters {
	return batch.Parameters{
		Max:      b.MaxParameters,
		Fixed:    b.FixedParameters,
		PerItem:  b.ParametersPerItem,
		MaxItems: b.MaxItems,
	}
}

func (b ParameterBatch) ItemLimit() (int, error) { return b.parameters().ItemLimit() }
func (b ParameterBatch) Ranges(total int) ([]BatchRange, error) {
	return b.parameters().Ranges(total)
}
