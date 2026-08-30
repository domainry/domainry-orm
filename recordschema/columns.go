package recordschema

import (
	"strings"

	"github.com/domainry/domainry-orm/schema"
)

var systemColumns = []schema.ColumnDefinition{
	schema.Column("workspace_id", schema.TextKey(255)).NotNull(),
	schema.Column("id", schema.TextKey(255)).NotNull(),
	schema.Column("created_at", schema.Timestamp()).NotNull().Default("current_timestamp"),
	schema.Column("updated_at", schema.Timestamp()).NotNull().Default("current_timestamp"),
	schema.Column("deleted", schema.Boolean()).NotNull().Default("false"),
	schema.Column("ext_info", schema.JSON()).NotNull().Default("empty_json"),
	schema.Column("create_by", schema.TextKey(255)),
	schema.Column("update_by", schema.TextKey(255)),
}

// SystemColumnNames returns the stable physical system-column inventory in
// migration order.
func SystemColumnNames() []string {
	return []string{"workspace_id", "id", "created_at", "updated_at", "deleted", "ext_info", "create_by", "update_by"}
}

// SystemColumn returns the canonical definition for one Record system column.
func SystemColumn(name string) (schema.ColumnDefinition, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for index, candidate := range SystemColumnNames() {
		if candidate == name {
			return systemColumns[index], true
		}
	}
	return schema.ColumnDefinition{}, false
}
