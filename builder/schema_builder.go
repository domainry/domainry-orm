package builder

import (
	"github.com/domainry/domainry-orm/recordschema"
	"github.com/domainry/domainry-orm/schema"
)

// Legacy DDL aliases keep existing modules source-compatible while schema
// declarations move out of the query builder package.
// Deprecated: import github.com/domainry/domainry-orm/schema.
type ColumnType = schema.ColumnType
type SchemaColumn = schema.ColumnDefinition
type CreateTableBuilder = schema.TableBuilder
type AddColumnBuilder = schema.AddColumnBuilder
type RenameColumnBuilder = schema.RenameColumnBuilder
type DropColumnBuilder = schema.DropColumnBuilder
type CreateIndexBuilder = schema.IndexBuilder
type PhysicalTable = schema.PhysicalTable
type PhysicalColumn = schema.PhysicalColumn

func TextType() ColumnType              { return schema.Text() }
func LongTextType() ColumnType          { return schema.LongText() }
func BigIntType() ColumnType            { return schema.BigInt() }
func IntegerType() ColumnType           { return schema.Integer() }
func SmallIntType() ColumnType          { return schema.SmallInt() }
func BooleanType() ColumnType           { return schema.Boolean() }
func RealType() ColumnType              { return schema.Real() }
func DoubleType() ColumnType            { return schema.Double() }
func DateType() ColumnType              { return schema.Date() }
func TimeType() ColumnType              { return schema.Time() }
func TimestampType() ColumnType         { return schema.Timestamp() }
func JSONType() ColumnType              { return schema.JSON() }
func UUIDType() ColumnType              { return schema.UUID() }
func BinaryType() ColumnType            { return schema.Binary() }
func DecimalType(p, s int) ColumnType   { return schema.Decimal(p, s) }
func VarcharType(length int) ColumnType { return schema.Varchar(length) }
func TextKeyType(length int) ColumnType { return schema.TextKey(length) }
func DefineColumn(name string, typ ColumnType) SchemaColumn {
	return schema.Column(name, typ)
}

// NewCreateTableBuilder preserves the historical Record-table default.
// Infrastructure tables should use schema.NewTable and Record tables should
// use recordschema.NewTable explicitly.
func NewCreateTableBuilder(renderer Renderer, table string) *CreateTableBuilder {
	return recordschema.NewTable(renderer, table)
}

func RecordSystemColumnNames() []string { return recordschema.SystemColumnNames() }
func RecordSystemColumn(name string) (SchemaColumn, bool) {
	return recordschema.SystemColumn(name)
}

func NewAddColumnBuilder(renderer Renderer, table string, column SchemaColumn) *AddColumnBuilder {
	return schema.NewAddColumn(renderer, table, column)
}

func NewDropColumnBuilder(renderer Renderer, table, column string) *DropColumnBuilder {
	return schema.NewDropColumn(renderer, table, column)
}

func NewRenameColumnBuilder(renderer Renderer, table, from, to string) *RenameColumnBuilder {
	return schema.NewRenameColumn(renderer, table, from, to)
}

func NewCreateIndexBuilder(renderer Renderer, index, table string) *CreateIndexBuilder {
	return schema.NewIndex(renderer, index, table)
}
