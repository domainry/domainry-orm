package builder_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
)

func TestFirstPageAppendsStableIDOrder(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.Postgres)
	statement, args, err := builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").
		Columns("id", "name").FirstPage(50, builder.KeysetAscending("name")).Build()
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
	statement, args, err := builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").
		Columns("id", "name").
		NextPage("user-9", 20, map[string]any{"name": "Zhao"}, builder.KeysetDescending("name")).Build()
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

	if _, _, err := builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").NextPage("", 20, nil).Build(); err == nil {
		t.Fatal("missing cursor ID was accepted")
	}
	if _, _, err := builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").NextPage("user-9", 20, nil, builder.KeysetAscending("name")).Build(); err == nil {
		t.Fatal("missing sort cursor value was accepted")
	}
}

func TestKeysetPaginationRejectsUnstableContracts(t *testing.T) {
	renderer := workspaceRenderer(t, dialect.SQLite)
	tests := []*builder.SelectBuilder{
		builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(0),
		builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(20, builder.KeysetOrder{Column: "name", Direction: "sideways"}),
		builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(20, builder.KeysetAscending("id"), builder.KeysetAscending("name")),
		builder.NewWorkspaceSelectBuilder(renderer, "users", "workspace-a").Columns("id").FirstPage(20, builder.KeysetAscending("name"), builder.KeysetDescending("name")),
	}
	for index, query := range tests {
		if _, _, err := query.Build(); err == nil {
			t.Fatalf("invalid keyset contract %d was accepted", index)
		}
	}
}
