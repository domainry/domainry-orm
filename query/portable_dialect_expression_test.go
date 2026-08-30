package query_test

import (
	"strings"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
)

func TestConcatRendersNativeParameterizedExpression(t *testing.T) {
	for _, testCase := range []struct {
		name dialect.Name
		want string
	}{
		{name: dialect.SQLite, want: `LOWER((COALESCE("event", ?) || ? || COALESCE("object_key", ?)))`},
		{name: dialect.Postgres, want: `LOWER((COALESCE("event", $1) || $2 || COALESCE("object_key", $3)))`},
		{name: dialect.MySQL, want: "LOWER(CONCAT(COALESCE(`event`, ?), ?, COALESCE(`object_key`, ?)))"},
	} {
		value, err := dialect.New(testCase.name)
		if err != nil {
			t.Fatal(err)
		}
		renderer, err := value.WithNamespace("", "")
		if err != nil {
			t.Fatal(err)
		}
		query, args, err := query.NewSelectBuilder(renderer, "audit_events").
			Projections(query.Project(query.Lower(query.Concat(
				query.Coalesce(query.Column("event"), query.Value("")),
				query.Value(" "),
				query.Coalesce(query.Column("object_key"), query.Value("")),
			)))).Build()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(query, testCase.want) || len(args) != 3 || args[0] != "" || args[1] != " " || args[2] != "" {
			t.Fatalf("dialect=%s query=%s args=%#v", testCase.name, query, args)
		}
	}
}

func TestConflictDoNothingIsPortableWithoutInsertIgnore(t *testing.T) {
	for _, name := range []dialect.Name{dialect.SQLite, dialect.Postgres, dialect.MySQL} {
		value, err := dialect.New(name)
		if err != nil {
			t.Fatal(err)
		}
		renderer, err := value.WithNamespace("", "")
		if err != nil {
			t.Fatal(err)
		}
		query, _, err := query.NewInsertBuilder(renderer, "seed").Columns("id", "value").Values("one", "value").OnConflictDoNothing("id").Build()
		if err != nil {
			t.Fatalf("dialect=%s: %v", name, err)
		}
		if strings.Contains(query, "INSERT IGNORE") {
			t.Fatalf("dialect=%s suppressed unrelated errors: %s", name, query)
		}
		if name == dialect.MySQL && !strings.Contains(query, "ON DUPLICATE KEY UPDATE `id` = `id`") {
			t.Fatalf("mysql query=%s", query)
		}
		if name != dialect.MySQL && !strings.Contains(query, "ON CONFLICT (\"") {
			t.Fatalf("dialect=%s query=%s", name, query)
		}
	}
}
