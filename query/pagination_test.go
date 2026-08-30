package query_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
)

func TestFirstPageAppendsStableIDOrder(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.Postgres)
	statement, args, err := query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").
		Columns("id", "name").FirstPage(50, query.KeysetAscending("name")).Build()
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT "id", "name" FROM "users" WHERE ("workspace_id" = $1) ORDER BY "name" ASC, "id" ASC LIMIT $2`
	if statement != want || !reflect.DeepEqual(args, []any{"workspace-a", 50}) {
		t.Fatalf("statement=%s args=%#v", statement, args)
	}
}

func TestNextPageBuildsCompositeKeysetAndRequiresID(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.Postgres)
	statement, args, err := query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").
		Columns("id", "name").
		NextPage("user-9", 20, map[string]any{"name": "Zhao"}, query.KeysetDescending("name")).Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`"workspace_id" = $1`,
		`(("name" < $2) OR ("name" = $3 AND "id" > $4))`,
		`ORDER BY "name" DESC, "id" ASC LIMIT $5`,
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("missing %q in %s", fragment, statement)
		}
	}
	if !reflect.DeepEqual(args, []any{"workspace-a", "Zhao", "Zhao", "user-9", 20}) {
		t.Fatalf("args=%#v", args)
	}

	if _, _, err := query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").NextPage("", 20, nil).Build(); err == nil {
		t.Fatal("missing cursor ID was accepted")
	}
	if _, _, err := query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").NextPage("user-9", 20, nil, query.KeysetAscending("name")).Build(); err == nil {
		t.Fatal("missing sort cursor value was accepted")
	}
}

func TestKeysetPaginationRejectsUnstableContracts(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.SQLite)
	tests := []*query.SelectBuilder{
		query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(0),
		query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(20, query.KeysetOrder{Column: "name", Direction: "sideways"}),
		query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(20, query.KeysetAscending("id"), query.KeysetAscending("name")),
		query.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(20, query.KeysetAscending("name"), query.KeysetDescending("name")),
	}
	for index, query := range tests {
		if _, _, err := query.Build(); err == nil {
			t.Fatalf("invalid keyset contract %d was accepted", index)
		}
	}
}
