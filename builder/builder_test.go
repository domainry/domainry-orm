package builder_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
)

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
			selectSQL, selectArgs, err := builder.NewSelectBuilder(renderer, "records").Columns("id", "name").Where(builder.And(builder.Equal("workspace_id", "workspace"), builder.In("status", "active", "pending"))).OrderBy(builder.Descending("created_at")).Limit(25).Offset(50).Build()
			if err != nil || selectSQL != test.selectSQL || !reflect.DeepEqual(selectArgs, []any{"workspace", "active", "pending", 25, 50}) {
				t.Fatalf("select=%q args=%v err=%v", selectSQL, selectArgs, err)
			}
			insertSQL, insertArgs, err := builder.NewInsertBuilder(renderer, "records").Columns("id", "name").Values("one", "First").Values("two", "Second").Build()
			if err != nil || insertSQL != test.insertSQL || !reflect.DeepEqual(insertArgs, []any{"one", "First", "two", "Second"}) {
				t.Fatalf("insert=%q args=%v err=%v", insertSQL, insertArgs, err)
			}
			updateSQL, updateArgs, err := builder.NewUpdateBuilder(renderer, "records").Set("name", "Updated").SetExpression("revision", builder.Add(builder.Column("revision"), builder.Value(1))).Where(builder.Equal("id", "one")).Build()
			if err != nil || updateSQL != test.updateSQL || !reflect.DeepEqual(updateArgs, []any{"Updated", 1, "one"}) {
				t.Fatalf("update=%q args=%v err=%v", updateSQL, updateArgs, err)
			}
			deleteSQL, deleteArgs, err := builder.NewDeleteBuilder(renderer, "records").Where(builder.Equal("id", "one")).Build()
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
		statement, args, err := builder.NewSelectBuilder(renderer, "items").Projections(
			builder.Project(builder.CountAll()),
			builder.Project(builder.Coalesce(builder.Sum(builder.CaseWhen(builder.Or(builder.Equal("state", "open"), builder.Equal("alert", "firing")), 1).Else(0)), builder.Value(0))),
			builder.Project(builder.Lower(builder.Column("search_text"))),
		).Build()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(statement, "COUNT(*)") || !strings.Contains(statement, "CASE WHEN") || !strings.Contains(statement, "LOWER(") {
			t.Fatalf("%s statement=%s", name, statement)
		}
		if len(args) != 5 {
			t.Fatalf("%s args=%v", name, args)
		}
	}
}

func TestMutatingBuildersRejectUnboundedStatements(t *testing.T) {
	renderer, _ := dialect.ParseRenderer("sqlite", "", "")
	if _, _, err := builder.NewUpdateBuilder(renderer, "records").Set("name", "unsafe").Build(); err == nil {
		t.Fatal("unbounded update was accepted")
	}
	if _, _, err := builder.NewDeleteBuilder(renderer, "records").Build(); err == nil {
		t.Fatal("unbounded delete was accepted")
	}
}

func TestBuilderValidationRejectsIncompleteStatements(t *testing.T) {
	renderer, _ := dialect.ParseRenderer("sqlite", "", "")
	if _, _, err := builder.NewInsertBuilder(renderer, "records").Columns("id", "name").Values("only-id").Build(); err == nil {
		t.Fatal("mismatched insert row was accepted")
	}
	if _, _, err := builder.NewSelectBuilder(renderer, "records").Columns("id").Where(builder.In("status")).Build(); err == nil {
		t.Fatal("empty set predicate was accepted")
	}
}
