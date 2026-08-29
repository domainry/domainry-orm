package builder_test

import (
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
)

func schemaRenderer(t *testing.T, name dialect.Name) dialect.Renderer {
	t.Helper()
	value, err := dialect.New(name)
	if err != nil {
		t.Fatal(err)
	}
	return value.WithSchema("")
}

func TestCreateTableBuilderRendersPortableKeyTypesAndConstraints(t *testing.T) {
	for _, name := range []dialect.Name{dialect.SQLite, dialect.Postgres, dialect.MySQL} {
		statement, args, err := builder.NewCreateTableBuilder(schemaRenderer(t, name), "agent_runs").IfNotExists().Columns(
			builder.DefineColumn("workspace_id", builder.TextKeyType(255)).NotNull(),
			builder.DefineColumn("run_id", builder.TextKeyType(255)).NotNull(),
			builder.DefineColumn("payload_json", builder.TextType()).NotNull(),
			builder.DefineColumn("updated_at", builder.BigIntType()).NotNull(),
		).PrimaryKey("workspace_id", "run_id").Unique("workspace_id", "run_id").Build()
		if err != nil || len(args) != 0 {
			t.Fatalf("dialect=%s statement=%s args=%v err=%v", name, statement, args, err)
		}
		if name == dialect.MySQL && !strings.Contains(statement, "VARCHAR(255)") {
			t.Fatalf("mysql key type=%s", statement)
		}
		if name != dialect.MySQL && strings.Contains(statement, "VARCHAR") {
			t.Fatalf("dialect=%s leaked mysql key type: %s", name, statement)
		}
		for _, required := range []string{"CREATE TABLE IF NOT EXISTS", "PRIMARY KEY", "UNIQUE", "payload_json", "BIGINT NOT NULL"} {
			if !strings.Contains(statement, required) {
				t.Fatalf("dialect=%s missing %q: %s", name, required, statement)
			}
		}
	}
}

func TestCreateTableBuilderRejectsInvalidDefinitions(t *testing.T) {
	renderer := schemaRenderer(t, dialect.SQLite)
	for _, build := range []func() error{
		func() error { _, _, err := builder.NewCreateTableBuilder(renderer, "empty").Build(); return err },
		func() error {
			_, _, err := builder.NewCreateTableBuilder(renderer, "duplicate").Columns(
				builder.DefineColumn("id", builder.TextType()), builder.DefineColumn("id", builder.TextType()),
			).Build()
			return err
		},
		func() error {
			_, _, err := builder.NewCreateTableBuilder(renderer, "missing_key").Columns(builder.DefineColumn("id", builder.TextType())).PrimaryKey("other").Build()
			return err
		},
	} {
		if err := build(); err == nil {
			t.Fatal("invalid create table definition accepted")
		}
	}
}

func TestCreateIndexBuilderKeepsDialectCapabilityExplicit(t *testing.T) {
	statement, _, err := builder.NewCreateIndexBuilder(schemaRenderer(t, dialect.Postgres), "idx_agent_claim", "agent_runs").
		Columns("workspace_id", "updated_at").IfNotExists().Build()
	if err != nil || !strings.Contains(statement, `CREATE INDEX IF NOT EXISTS "idx_agent_claim"`) {
		t.Fatalf("postgres index=%s err=%v", statement, err)
	}
	if _, _, err := builder.NewCreateIndexBuilder(schemaRenderer(t, dialect.MySQL), "idx_agent_claim", "agent_runs").Columns("workspace_id").IfNotExists().Build(); err == nil {
		t.Fatal("mysql IF NOT EXISTS index capability was silently accepted")
	}
}
