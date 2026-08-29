package builder

import "fmt"

// BatchRange is one half-open item range [Start, End).
type BatchRange struct {
	Start int
	End   int
}

// ParameterBatch centralizes safe SQL batching from the database parameter
// budget. FixedParameters are present once per statement; ParametersPerItem
// are contributed by every item. MaxItems additionally caps operational page
// size even when the engine allows more parameters.
type ParameterBatch struct {
	MaxParameters     int
	FixedParameters   int
	ParametersPerItem int
	MaxItems          int
}

func (b ParameterBatch) ItemLimit() (int, error) {
	if b.MaxParameters <= 0 || b.FixedParameters < 0 || b.ParametersPerItem <= 0 || b.FixedParameters >= b.MaxParameters {
		return 0, fmt.Errorf("SQL parameter batch configuration is invalid")
	}
	limit := (b.MaxParameters - b.FixedParameters) / b.ParametersPerItem
	if b.MaxItems > 0 && b.MaxItems < limit {
		limit = b.MaxItems
	}
	if limit <= 0 {
		return 0, fmt.Errorf("SQL parameter batch has no item capacity")
	}
	return limit, nil
}

func (b ParameterBatch) Ranges(total int) ([]BatchRange, error) {
	if total < 0 {
		return nil, fmt.Errorf("SQL parameter batch total is invalid")
	}
	limit, err := b.ItemLimit()
	if err != nil || total == 0 {
		return []BatchRange{}, err
	}
	ranges := make([]BatchRange, 0, (total+limit-1)/limit)
	for start := 0; start < total; start += limit {
		ranges = append(ranges, BatchRange{Start: start, End: min(start+limit, total)})
	}
	return ranges, nil
}
