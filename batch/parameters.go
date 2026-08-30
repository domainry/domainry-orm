package batch

import "fmt"

// Range is one half-open item range [Start, End).
type Range struct {
	Start int
	End   int
}

// Parameters centralizes safe SQL batching from the database parameter
// budget. Fixed are present once per statement; PerItem are contributed by
// every item. MaxItems additionally caps operational page size.
type Parameters struct {
	Max      int
	Fixed    int
	PerItem  int
	MaxItems int
}

func (b Parameters) ItemLimit() (int, error) {
	if b.Max <= 0 || b.Fixed < 0 || b.PerItem <= 0 || b.Fixed >= b.Max {
		return 0, fmt.Errorf("SQL parameter batch configuration is invalid")
	}
	limit := (b.Max - b.Fixed) / b.PerItem
	if b.MaxItems > 0 && b.MaxItems < limit {
		limit = b.MaxItems
	}
	if limit <= 0 {
		return 0, fmt.Errorf("SQL parameter batch has no item capacity")
	}
	return limit, nil
}

func (b Parameters) Ranges(total int) ([]Range, error) {
	if total < 0 {
		return nil, fmt.Errorf("SQL parameter batch total is invalid")
	}
	limit, err := b.ItemLimit()
	if err != nil || total == 0 {
		return []Range{}, err
	}
	ranges := make([]Range, 0, (total+limit-1)/limit)
	for start := 0; start < total; start += limit {
		ranges = append(ranges, Range{Start: start, End: min(start+limit, total)})
	}
	return ranges, nil
}
