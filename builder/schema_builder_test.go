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
			builder.DefineColumn("run_id", builder.TextKeyType(255)).NotNull(),
			builder.DefineColumn("payload_json", builder.TextType()).NotNull(),
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
		for _, required := range []string{"CREATE TABLE IF NOT EXISTS", "PRIMARY KEY", "UNIQUE", "payload_json", "workspace_id", "id", "created_at", "updated_at", "deleted", "ext_info", "create_user_id", "update_user_id"} {
			if !strings.Contains(statement, required) {
				t.Fatalf("dialect=%s missing %q: %s", name, required, statement)
			}
		}
	}
}

func TestCreateTableBuilderRejectsInvalidDefinitions(t *testing.T) {
	renderer := schemaRenderer(t, dialect.SQLite)
	for _, build := range []func() error{
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

func TestCreateTableBuilderAddsSystemColumnsWithPortableTypesAndDefaults(t *testing.T) {
	for _, name := range []dialect.Name{dialect.SQLite, dialect.Postgres, dialect.MySQL} {
		statement, _, err := builder.NewCreateTableBuilder(schemaRenderer(t, name), "customer").Columns(
			builder.DefineColumn("balance", builder.DecimalType(19, 4)),
			builder.DefineColumn("profile", builder.JSONType()),
			builder.DefineColumn("external_id", builder.UUIDType()),
			builder.DefineColumn("attachment", builder.BinaryType()),
		).Build()
		if err != nil {
			t.Fatalf("dialect=%s err=%v", name, err)
		}
		for _, required := range []string{"DECIMAL(19,4)", "workspace_id", "id", "TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP", "deleted", "ext_info", "create_user_id", "update_user_id"} {
			if !strings.Contains(statement, required) {
				t.Fatalf("dialect=%s missing %q: %s", name, required, statement)
			}
		}
		switch name {
		case dialect.SQLite:
			for _, required := range []string{"BLOB", `DEFAULT '{}'`, "BOOLEAN NOT NULL DEFAULT 0"} {
				if !strings.Contains(statement, required) {
					t.Fatalf("sqlite missing %q: %s", required, statement)
				}
			}
		case dialect.Postgres:
			for _, required := range []string{"JSONB", "UUID", "BYTEA", `DEFAULT '{}'::jsonb`} {
				if !strings.Contains(statement, required) {
					t.Fatalf("postgres missing %q: %s", required, statement)
				}
			}
		case dialect.MySQL:
			for _, required := range []string{"JSON", "VARCHAR(36)", "LONGBLOB", "DEFAULT (JSON_OBJECT())"} {
				if !strings.Contains(statement, required) {
					t.Fatalf("mysql missing %q: %s", required, statement)
				}
			}
		}
	}
}

func TestCreateTableBuilderAcceptsSystemOnlyTableAndRejectsInvalidDecimal(t *testing.T) {
	renderer := schemaRenderer(t, dialect.SQLite)
	statement, _, err := builder.NewCreateTableBuilder(renderer, "system_only").Build()
	if err != nil || !strings.Contains(statement, `"update_user_id"`) {
		t.Fatalf("system-only statement=%s err=%v", statement, err)
	}
	if _, _, err := builder.NewCreateTableBuilder(renderer, "bad_decimal").Columns(
		builder.DefineColumn("amount", builder.DecimalType(2, 3)),
	).Build(); err == nil {
		t.Fatal("invalid decimal accepted")
	}
}

func TestCreateTableBuilderCanDeclareInfrastructureSchema(t *testing.T) {
	statement, _, err := builder.NewCreateTableBuilder(schemaRenderer(t, dialect.SQLite), "agent_task_runs").
		WithoutSystemColumns().Columns(
		builder.DefineColumn("workspace_id", builder.TextKeyType(255)).NotNull(),
		builder.DefineColumn("run_id", builder.TextKeyType(255)).NotNull(),
		builder.DefineColumn("updated_at", builder.BigIntType()).NotNull(),
	).PrimaryKey("workspace_id", "run_id").Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"id"`, `"deleted"`, `"ext_info"`, `"create_user_id"`, `"update_user_id"`} {
		if strings.Contains(statement, forbidden) {
			t.Fatalf("infrastructure schema contains %s: %s", forbidden, statement)
		}
	}
	if _, _, err := builder.NewCreateTableBuilder(schemaRenderer(t, dialect.SQLite), "empty_infrastructure").WithoutSystemColumns().Build(); err == nil {
		t.Fatal("empty infrastructure schema accepted")
	}
}

func TestRecordSystemColumnsDriveExistingTableMigration(t *testing.T) {
	names := builder.RecordSystemColumnNames()
	if len(names) != 8 || names[0] != "workspace_id" || names[len(names)-1] != "update_user_id" {
		t.Fatalf("system columns=%v", names)
	}
	column, ok := builder.RecordSystemColumn("deleted")
	if !ok {
		t.Fatal("deleted system column missing")
	}
	statement, args, err := builder.NewAddColumnBuilder(schemaRenderer(t, dialect.Postgres), "customer", column).Build()
	if err != nil || len(args) != 0 || statement != `ALTER TABLE "customer" ADD COLUMN "deleted" BOOLEAN NOT NULL DEFAULT FALSE` {
		t.Fatalf("statement=%s args=%v err=%v", statement, args, err)
	}
	if _, ok := builder.RecordSystemColumn("unknown"); ok {
		t.Fatal("unknown system column accepted")
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
