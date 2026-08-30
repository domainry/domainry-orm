package query_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
)

func workspaceRenderer(t *testing.T, name dialect.Name) dialect.Renderer {
	t.Helper()
	d, err := dialect.New(name)
	if err != nil {
		t.Fatal(err)
	}
	return d.WithSchema("")
}

func TestWorkspaceBuildersKeepMandatoryScope(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.Postgres)
	statement, args, err := query.NewWorkspaceSelectBuilder(renderer, "orders", "workspace-a").
		Columns("id").Where(query.Equal("status", "open")).Build()
	if err != nil {
		t.Fatal(err)
	}
	if statement != `SELECT "id" FROM "orders" WHERE ("workspace_id" = $1 AND "status" = $2)` || !reflect.DeepEqual(args, []any{"workspace-a", "open"}) {
		t.Fatalf("select=%s args=%#v", statement, args)
	}

	statement, args, err = query.NewWorkspaceInsertBuilder(renderer, "orders", "workspace-a").
		Columns("id", "status").Values("order-1", "open").Build()
	if err != nil {
		t.Fatal(err)
	}
	if statement != `INSERT INTO "orders" ("workspace_id", "id", "status") VALUES ($1, $2, $3)` || !reflect.DeepEqual(args, []any{"workspace-a", "order-1", "open"}) {
		t.Fatalf("insert=%s args=%#v", statement, args)
	}

	statement, args, err = query.NewWorkspaceUpdateBuilder(renderer, "orders", "workspace-a").
		Set("status", "closed").Where(query.Equal("id", "order-1")).Build()
	if err != nil || !strings.Contains(statement, `WHERE ("workspace_id" = $2 AND "id" = $3)`) || !reflect.DeepEqual(args, []any{"closed", "workspace-a", "order-1"}) {
		t.Fatalf("update=%s args=%#v err=%v", statement, args, err)
	}

	statement, args, err = query.NewWorkspaceDeleteBuilder(renderer, "orders", "workspace-a").
		Where(query.Equal("id", "order-1")).Build()
	if err != nil || !strings.Contains(statement, `WHERE ("workspace_id" = $1 AND "id" = $2)`) || !reflect.DeepEqual(args, []any{"workspace-a", "order-1"}) {
		t.Fatalf("delete=%s args=%#v err=%v", statement, args, err)
	}
}

func TestWorkspaceBuildersRejectMissingWorkspaceAndOffsetPagination(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.SQLite)
	if _, _, err := query.NewWorkspaceSelectBuilder(renderer, "orders", " ").Columns("id").Build(); err == nil {
		t.Fatal("missing workspaceID was accepted")
	}
	if _, _, err := query.NewWorkspaceInsertBuilder(renderer, "orders", "").Columns("id").Values("one").Build(); err == nil {
		t.Fatal("missing insert workspaceID was accepted")
	}
	if _, _, err := query.NewWorkspaceSelectBuilder(renderer, "orders", "workspace-a").Columns("id").Offset(5000).Build(); err == nil || !strings.Contains(err.Error(), "ID cursor") {
		t.Fatalf("workspace OFFSET err=%v", err)
	}
	if _, _, err := query.NewWorkspaceSelectBuilder(renderer, "orders", "workspace-a").Columns("id").AfterID(" ").Limit(100).Build(); err == nil || !strings.Contains(err.Error(), "ID cursor") {
		t.Fatalf("empty cursor err=%v", err)
	}
}

func TestWorkspaceKeysetPaginationBindsIDAndLimit(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.Postgres)
	statement, args, err := query.NewWorkspaceSelectBuilder(renderer, "orders", "workspace-a").
		Columns("id").AfterID("order-1000").Limit(100).Build()
	if err != nil {
		t.Fatal(err)
	}
	if statement != `SELECT "id" FROM "orders" WHERE ("workspace_id" = $1 AND "id" > $2) ORDER BY "id" ASC LIMIT $3` || !reflect.DeepEqual(args, []any{"workspace-a", "order-1000", 100}) {
		t.Fatalf("keyset=%s args=%#v", statement, args)
	}
}

func TestWorkspaceSelectQualifiesMandatoryScopeWhenBaseTableHasAlias(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.Postgres)
	statement, args, err := query.NewWorkspaceSelectBuilder(renderer, "orders", "workspace-a").
		Alias("o").
		Projections(query.Project(query.QualifiedColumn("o", "id"))).
		Join(query.InnerJoin("order_items", "i", query.EqualExpressions(
			query.QualifiedColumn("i", "order_id"), query.QualifiedColumn("o", "id"),
		))).
		Where(query.EqualValue(query.QualifiedColumn("i", "status"), "open")).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "o"."id" FROM "orders" AS "o" INNER JOIN "order_items" AS "i" ON "i"."order_id" = "o"."id" WHERE ("o"."workspace_id" = $1 AND "i"."status" = $2)`
	if statement != want || !reflect.DeepEqual(args, []any{"workspace-a", "open"}) {
		t.Fatalf("statement=%s args=%#v", statement, args)
	}
}

func TestInsertedValueRendersPerDialectWithoutRawSQL(t *testing.T) {
	for _, test := range []struct {
		name dialect.Name
		want string
	}{
		{dialect.MySQL, "ON DUPLICATE KEY UPDATE `value` = VALUES(`value`)"},
		{dialect.Postgres, `ON CONFLICT ("key") DO UPDATE SET "value" = "excluded"."value"`},
		{dialect.SQLite, `ON CONFLICT ("key") DO UPDATE SET "value" = "excluded"."value"`},
	} {
		renderer := workspaceRenderer(t, test.name)
		insert := query.NewInsertBuilder(renderer, "catalog").Columns("key", "value").Values("key", "value")
		if test.name == dialect.MySQL {
			insert.OnDuplicateKeyUpdate(query.AssignExpression("value", query.InsertedValue("value")))
		} else {
			insert.OnConflictDoUpdate([]string{"key"}, query.AssignExpression("value", query.InsertedValue("value")))
		}
		statement, _, err := insert.Build()
		if err != nil || !strings.Contains(statement, test.want) {
			t.Fatalf("%s statement=%s err=%v", test.name, statement, err)
		}
	}
}
