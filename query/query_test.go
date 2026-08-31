package query_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
)

type legacyRenderer struct{}

func (legacyRenderer) Identifier(value string) string { return `"` + value + `"` }
func (legacyRenderer) Table(value string) string      { return `"` + value + `"` }
func (legacyRenderer) Placeholder(int) string         { return "?" }

func TestBuilderAcceptsRendererWithoutDialectName(t *testing.T) {
	queryValue, args, err := query.NewSelectBuilder(legacyRenderer{}, "records").Columns("id").Where(query.Equal("id", "one")).Build()
	if err != nil {
		t.Fatal(err)
	}
	if queryValue != `SELECT "id" FROM "records" WHERE "id" = ?` || len(args) != 1 || args[0] != "one" {
		t.Fatalf("unexpected legacy renderer result: %s %#v", queryValue, args)
	}
}

func TestPublishedExpressionCompatibility(t *testing.T) {
	renderer, err := dialect.ParseRenderer("postgres", "runtime", "")
	if err != nil {
		t.Fatal(err)
	}
	queryValue, args, err := query.NewSelectBuilder(renderer, "records").Projections(
		query.Project(query.TableColumn("records", "id")), query.Project(query.AllColumns()),
	).Build()
	if err != nil {
		t.Fatal(err)
	}
	if queryValue != `SELECT "runtime"."records"."id", * FROM "runtime"."records"` || len(args) != 0 {
		t.Fatalf("unexpected compatibility expression result: %s %#v", queryValue, args)
	}
}

func TestPrepareSQLFragments(t *testing.T) {
	renderer := postgresRenderer(t)
	predicate, args, err := query.PreparePredicate(renderer, query.And(
		query.Equal("workspace_id", "workspace"), query.In("status", "open", "closed"),
	), 3)
	if err != nil {
		t.Fatal(err)
	}
	if predicate != `("workspace_id" = $4 AND "status" IN ($5, $6))` || len(args) != 3 {
		t.Fatalf("unexpected prepared predicate: %s %#v", predicate, args)
	}
	order, err := query.PrepareOrderBy(renderer, query.Descending("created_at"), query.Ascending("id"))
	if err != nil {
		t.Fatal(err)
	}
	if order != `ORDER BY "created_at" DESC, "id" ASC` {
		t.Fatalf("unexpected prepared order: %s", order)
	}
}

func TestStructuredPredicateExtensions(t *testing.T) {
	renderer := sqliteRenderer(t)
	predicate := query.And(
		query.Not(query.Equal("status", "deleted")),
		query.IsNotNullExpression(query.QualifiedColumn("records", "owner_id")),
		query.LikeValueEscaped(query.Lower(query.Column("name")), "a~_%"),
		query.AlwaysFalse(),
	)
	prepared, args, err := query.PreparePredicate(renderer, predicate, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := `(NOT ("status" = ?) AND "records"."owner_id" IS NOT NULL AND LOWER("name") LIKE ? ESCAPE '~' AND 1 = 0)`
	if prepared != want || len(args) != 2 {
		t.Fatalf("unexpected structured predicate: %s %#v", prepared, args)
	}
}

func TestBuildersRenderAcrossSupportedDialects(t *testing.T) {
	tests := []struct {
		name      string
		selectSQL string
		insertSQL string
		updateSQL string
		deleteSQL string
	}{
		{"sqlite", `SELECT "id", "name" FROM "records" WHERE ("workspace_id" = ? AND "status" IN (?, ?)) ORDER BY "created_at" DESC LIMIT ? OFFSET ?`, `INSERT INTO "records" ("id", "name") VALUES (?, ?), (?, ?)`, `UPDATE "records" SET "name" = ?, "revision" = "revision" + ? WHERE "id" = ?`, `DELETE FROM "records" WHERE "id" = ?`},
		{"mysql", "SELECT `id`, `name` FROM `records` WHERE (`workspace_id` = ? AND `status` IN (?, ?)) ORDER BY `created_at` DESC LIMIT ? OFFSET ?", "INSERT INTO `records` (`id`, `name`) VALUES (?, ?), (?, ?)", "UPDATE `records` SET `name` = ?, `revision` = `revision` + ? WHERE `id` = ?", "DELETE FROM `records` WHERE `id` = ?"},
		{"postgres", `SELECT "id", "name" FROM "records" WHERE ("workspace_id" = $1 AND "status" IN ($2, $3)) ORDER BY "created_at" DESC LIMIT $4 OFFSET $5`, `INSERT INTO "records" ("id", "name") VALUES ($1, $2), ($3, $4)`, `UPDATE "records" SET "name" = $1, "revision" = "revision" + $2 WHERE "id" = $3`, `DELETE FROM "records" WHERE "id" = $1`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renderer, err := dialect.ParseRenderer(test.name, "", "")
			if err != nil {
				t.Fatal(err)
			}
			selectSQL, selectArgs, err := query.NewSelectBuilder(renderer, "records").Columns("id", "name").Where(query.And(query.Equal("workspace_id", "workspace"), query.In("status", "active", "pending"))).OrderBy(query.Descending("created_at")).Limit(25).Offset(50).Build()
			if err != nil || selectSQL != test.selectSQL || !reflect.DeepEqual(selectArgs, []any{"workspace", "active", "pending", 25, 50}) {
				t.Fatalf("select=%q args=%v err=%v", selectSQL, selectArgs, err)
			}
			insertSQL, insertArgs, err := query.NewInsertBuilder(renderer, "records").Columns("id", "name").Values("one", "First").Values("two", "Second").Build()
			if err != nil || insertSQL != test.insertSQL || !reflect.DeepEqual(insertArgs, []any{"one", "First", "two", "Second"}) {
				t.Fatalf("insert=%q args=%v err=%v", insertSQL, insertArgs, err)
			}
			updateSQL, updateArgs, err := query.NewUpdateBuilder(renderer, "records").Set("name", "Updated").SetExpression("revision", query.Add(query.Column("revision"), query.Value(1))).Where(query.Equal("id", "one")).Build()
			if err != nil || updateSQL != test.updateSQL || !reflect.DeepEqual(updateArgs, []any{"Updated", 1, "one"}) {
				t.Fatalf("update=%q args=%v err=%v", updateSQL, updateArgs, err)
			}
			deleteSQL, deleteArgs, err := query.NewDeleteBuilder(renderer, "records").Where(query.Equal("id", "one")).Build()
			if err != nil || deleteSQL != test.deleteSQL || !reflect.DeepEqual(deleteArgs, []any{"one"}) {
				t.Fatalf("delete=%q args=%v err=%v", deleteSQL, deleteArgs, err)
			}
		})
	}
}

func TestSelectBuilderAggregateCaseAndLower(t *testing.T) {
	for _, name := range []string{"sqlite", "mysql", "postgres"} {
		renderer, err := dialect.ParseRenderer(name, "", "")
		if err != nil {
			t.Fatal(err)
		}
		statement, args, err := query.NewSelectBuilder(renderer, "items").Projections(
			query.Project(query.CountAll()),
			query.Project(query.Coalesce(query.Sum(query.CaseWhen(query.Or(query.Equal("state", "open"), query.Equal("alert", "firing")), 1).Else(0)), query.Value(0))),
			query.Project(query.Lower(query.Column("search_text"))),
		).OrderBy(query.DescendingExpression(query.CountAll())).Build()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(statement, "COUNT(*)") || !strings.Contains(statement, "CASE WHEN") || !strings.Contains(statement, "LOWER(") || !strings.Contains(statement, "ORDER BY COUNT(*) DESC") {
			t.Fatalf("%s statement=%s", name, statement)
		}
		if len(args) != 5 {
			t.Fatalf("%s args=%v", name, args)
		}
	}
}

func TestMutatingBuildersRejectUnboundedStatements(t *testing.T) {
	renderer, _ := dialect.ParseRenderer("sqlite", "", "")
	if _, _, err := query.NewUpdateBuilder(renderer, "records").Set("name", "unsafe").Build(); err == nil {
		t.Fatal("unbounded update was accepted")
	}
	if _, _, err := query.NewDeleteBuilder(renderer, "records").Build(); err == nil {
		t.Fatal("unbounded delete was accepted")
	}
}

func TestBuilderValidationRejectsIncompleteStatements(t *testing.T) {
	renderer, _ := dialect.ParseRenderer("sqlite", "", "")
	if _, _, err := query.NewInsertBuilder(renderer, "records").Columns("id", "name").Values("only-id").Build(); err == nil {
		t.Fatal("mismatched insert row was accepted")
	}
	if _, _, err := query.NewSelectBuilder(renderer, "records").Columns("id").Where(query.In("status")).Build(); err == nil {
		t.Fatal("empty set predicate was accepted")
	}
}
