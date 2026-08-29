package migration

type Baseline struct {
	Tables []Table
}

type Table struct {
	Name    string
	Columns []Column
	Indexes []Index
}

type Column struct {
	Name       string
	Type       string
	Nullable   bool
	PrimaryKey bool
}

type Index struct {
	Name    string
	Unique  bool
	Columns []string
}

// Migration is source-owned DDL applied by a database host. Baseline allows a
// host to adopt an existing schema only after proving its physical shape.
type Migration struct {
	Version    uint
	Name       string
	Statements []string
	Baseline   *Baseline
}
