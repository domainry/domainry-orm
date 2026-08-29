package builder

import (
	"fmt"
	"strings"
)

type Order struct {
	column    string
	direction string
}

func Ascending(column string) Order  { return Order{column: column, direction: "ASC"} }
func Descending(column string) Order { return Order{column: column, direction: "DESC"} }

func (o Order) render(renderer Renderer) (string, error) {
	if strings.TrimSpace(o.column) == "" {
		return "", fmt.Errorf("SQL order column is required")
	}
	return renderer.Identifier(o.column) + " " + o.direction, nil
}
