package schema_test

import (
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/schema"
)

func TestTableContainsOnlyDeclaredColumns(t *testing.T) {
	renderer, err := dialect.ParseRenderer("sqlite", "", "")
	if err != nil {
		t.Fatal(err)
	}
	statement, args, err := schema.NewTable(renderer, "migration_lock").
		Columns(schema.Column("name", schema.TextKey(255)).NotNull()).
		PrimaryKey("name").
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 0 {
		t.Fatalf("DDL must not bind arguments: %#v", args)
	}
	if strings.Contains(statement, "workspace_id") || strings.Contains(statement, "created_at") {
		t.Fatalf("generic schema declaration injected Record columns: %s", statement)
	}
	if want := `CREATE TABLE "migration_lock" ("name" TEXT NOT NULL, PRIMARY KEY ("name"))`; statement != want {
		t.Fatalf("unexpected table declaration:\n got: %s\nwant: %s", statement, want)
	}
}

func TestEmptyGenericTableIsRejected(t *testing.T) {
	renderer, err := dialect.ParseRenderer("postgres", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := schema.NewTable(renderer, "empty").Build(); err == nil {
		t.Fatal("expected an empty generic table to be rejected")
	}
}

func TestPortableColumnTypeRendering(t *testing.T) {
	for _, name := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(name, func(t *testing.T) {
			renderer, err := dialect.ParseRenderer(name, "", "")
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = schema.NewTable(renderer, "documents").Columns(
				schema.Column("id", schema.UUID()).NotNull(),
				schema.Column("payload", schema.JSON()).NotNull().Default("empty_json"),
				schema.Column("content", schema.Binary()),
			).PrimaryKey("id").Build()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
