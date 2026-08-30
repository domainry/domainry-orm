package query

import (
	"fmt"
	"strings"
)

// KeysetOrder defines one stable keyset ordering component. ID is mandatory
// as the final component; it is appended ascending when callers omit it.
type KeysetOrder struct {
	Column    string
	Direction string
}

func KeysetAscending(column string) KeysetOrder {
	return KeysetOrder{Column: strings.TrimSpace(column), Direction: "ASC"}
}

func KeysetDescending(column string) KeysetOrder {
	return KeysetOrder{Column: strings.TrimSpace(column), Direction: "DESC"}
}

// FirstPage applies deterministic ordering and a bounded page size without a
// cursor. Subsequent pages must use NextPage rather than OFFSET.
func (b *SelectBuilder) FirstPage(limit int, orders ...KeysetOrder) *SelectBuilder {
	normalized, err := normalizeKeysetOrders(orders)
	if err != nil {
		b.buildError = err
		return b
	}
	if limit <= 0 {
		b.buildError = fmt.Errorf("SQL keyset page requires a positive limit")
		return b
	}
	b.orders = keysetSQLOrders(normalized)
	b.limit = limit
	return b
}

// NextPage applies a lexicographic keyset predicate. cursorID is required and
// supplies the final ID value. values must contain each preceding sort value,
// normally resolved from the cursor row inside the same workspace.
func (b *SelectBuilder) NextPage(cursorID string, limit int, values map[string]any, orders ...KeysetOrder) *SelectBuilder {
	cursorID = strings.TrimSpace(cursorID)
	if cursorID == "" {
		b.buildError = fmt.Errorf("SQL keyset next page requires cursor ID")
		return b
	}
	normalized, err := normalizeKeysetOrders(orders)
	if err != nil {
		b.buildError = err
		return b
	}
	if limit <= 0 {
		b.buildError = fmt.Errorf("SQL keyset page requires a positive limit")
		return b
	}
	cursorValues := make([]any, len(normalized))
	for index, order := range normalized {
		if order.Column == "id" {
			cursorValues[index] = cursorID
			continue
		}
		value, found := values[order.Column]
		if !found || value == nil {
			b.buildError = fmt.Errorf("SQL keyset cursor requires value for %s", order.Column)
			return b
		}
		cursorValues[index] = value
	}
	b.required = append(b.required, keysetAfterPredicate(normalized, cursorValues))
	b.orders = keysetSQLOrders(normalized)
	b.limit = limit
	return b
}

func normalizeKeysetOrders(orders []KeysetOrder) ([]KeysetOrder, error) {
	normalized := make([]KeysetOrder, 0, len(orders)+1)
	seen := map[string]bool{}
	for _, order := range orders {
		column := strings.TrimSpace(order.Column)
		direction := strings.ToUpper(strings.TrimSpace(order.Direction))
		if column == "" || direction != "ASC" && direction != "DESC" {
			return nil, fmt.Errorf("SQL keyset order requires a column and ASC or DESC direction")
		}
		if seen[column] {
			return nil, fmt.Errorf("SQL keyset order column %s is duplicated", column)
		}
		if column == "id" && len(normalized) != len(orders)-1 {
			return nil, fmt.Errorf("SQL keyset ID must be the final order")
		}
		seen[column] = true
		normalized = append(normalized, KeysetOrder{Column: column, Direction: direction})
	}
	if !seen["id"] {
		normalized = append(normalized, KeysetAscending("id"))
	}
	return normalized, nil
}

func keysetSQLOrders(orders []KeysetOrder) []Order {
	result := make([]Order, len(orders))
	for index, order := range orders {
		if order.Direction == "DESC" {
			result[index] = Descending(order.Column)
		} else {
			result[index] = Ascending(order.Column)
		}
	}
	return result
}

func keysetAfterPredicate(orders []KeysetOrder, values []any) Predicate {
	branches := make([]Predicate, 0, len(orders))
	for index, order := range orders {
		terms := make([]Predicate, 0, index+1)
		for previous := 0; previous < index; previous++ {
			terms = append(terms, EqualValue(Column(orders[previous].Column), values[previous]))
		}
		if order.Direction == "DESC" {
			terms = append(terms, LessThanValue(Column(order.Column), values[index]))
		} else {
			terms = append(terms, GreaterThanExpression(Column(order.Column), values[index]))
		}
		branches = append(branches, And(terms...))
	}
	return Or(branches...)
}
