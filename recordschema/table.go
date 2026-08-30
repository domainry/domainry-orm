package recordschema

import "github.com/domainry/domainry-orm/schema"

// NewTable declares a Domainry Record table. The canonical system columns are
// merged after source-owned columns and cannot be omitted accidentally.
func NewTable(renderer schema.Renderer, table string) *schema.TableBuilder {
	return schema.NewTableWithRequiredColumns(renderer, table, systemColumns...)
}
