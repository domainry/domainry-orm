package schema

import (
	"fmt"
	"strings"
)

type PhysicalTable struct {
	Name    string
	Columns []PhysicalColumn
}

type PhysicalColumn struct {
	Name       string
	Type       string
	Nullable   bool
	PrimaryKey bool
}

// PhysicalTable projects the exact physical table shape from the same
// portable declaration used by Build. Embedded modules use it to prove and
// adopt source-owned tables that predate module extraction without duplicating
// dialect-specific column-type knowledge.
func (b *TableBuilder) PhysicalTable() (PhysicalTable, error) {
	if b == nil || b.renderer == nil || b.table == "" {
		return PhysicalTable{}, fmt.Errorf("SQL physical table requires renderer and table")
	}
	named, ok := b.renderer.(namedRenderer)
	if !ok {
		return PhysicalTable{}, fmt.Errorf("SQL physical table requires a named dialect renderer")
	}
	columns, err := mergeRequiredColumns(b.columns, b.requiredColumns)
	if err != nil {
		return PhysicalTable{}, err
	}
	primary := map[string]struct{}{}
	for _, constraint := range b.constraints {
		if constraint.kind == "PRIMARY KEY" {
			for _, column := range constraint.columns {
				primary[strings.ToLower(strings.TrimSpace(column))] = struct{}{}
			}
		}
	}
	result := PhysicalTable{Name: b.table, Columns: make([]PhysicalColumn, len(columns))}
	for index, column := range columns {
		if column.name == "" || column.typeOf == nil {
			return PhysicalTable{}, fmt.Errorf("SQL physical table column requires name and type")
		}
		physical, err := column.typeOf.renderColumnType(named.Name())
		if err != nil {
			return PhysicalTable{}, err
		}
		_, isPrimary := primary[strings.ToLower(column.name)]
		result.Columns[index] = PhysicalColumn{Name: column.name, Type: physical, Nullable: !column.notNull, PrimaryKey: isPrimary}
	}
	return result, nil
}

func mergeRequiredColumns(columns, requiredColumns []ColumnDefinition) ([]ColumnDefinition, error) {
	result := append([]ColumnDefinition(nil), columns...)
	existing := make(map[string]ColumnDefinition, len(columns))
	for _, column := range columns {
		key := strings.ToLower(column.name)
		if _, found := existing[key]; found {
			return nil, fmt.Errorf("SQL table column %q is duplicated", column.name)
		}
		existing[key] = column
	}
	for _, required := range requiredColumns {
		if _, found := existing[required.name]; !found {
			result = append(result, required)
		}
	}
	return result, nil
}
