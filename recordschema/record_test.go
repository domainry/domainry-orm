package recordschema_test

import (
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/recordschema"
	"github.com/domainry/domainry-orm/schema"
)

func TestRecordTableInjectsCanonicalSystemColumns(t *testing.T) {
	renderer, err := dialect.ParseRenderer("sqlite", "", "")
	if err != nil {
		t.Fatal(err)
	}
	statement, _, err := recordschema.NewTable(renderer, "customers").
		Columns(schema.Column("display_name", schema.Text()).NotNull()).
		PrimaryKey("workspace_id", "id").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{"display_name"}, recordschema.SystemColumnNames()...) {
		if !strings.Contains(statement, renderer.Identifier(name)) {
			t.Fatalf("Record table is missing %q: %s", name, statement)
		}
	}
	if strings.Index(statement, `"display_name"`) > strings.Index(statement, `"workspace_id"`) {
		t.Fatalf("source-owned columns must precede convention columns: %s", statement)
	}
}

func TestSystemColumnReturnsDefensiveValue(t *testing.T) {
	column, ok := recordschema.SystemColumn(" UPDATED_AT ")
	if !ok {
		t.Fatal("expected canonical updated_at column")
	}
	renderer, _ := dialect.ParseRenderer("postgres", "", "")
	if _, _, err := schema.NewAddColumn(renderer, "customers", column).Build(); err != nil {
		t.Fatal(err)
	}
	if _, ok := recordschema.SystemColumn("unknown"); ok {
		t.Fatal("unexpected unknown system column")
	}
}
