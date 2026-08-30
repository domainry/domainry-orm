package query_test

import (
	"reflect"
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
)

func sqliteRenderer(t *testing.T) dialect.Renderer {
	t.Helper()
	renderer, err := dialect.ParseRenderer("sqlite", "", "")
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return renderer
}

func postgresRenderer(t *testing.T) dialect.Renderer {
	t.Helper()
	renderer, err := dialect.ParseRenderer("postgres", "", "")
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return renderer
}

// assertSQL fails the test unless Build returns the expected statement, args and
// a nil error.
func assertSQL(t *testing.T, sql string, args []any, err error, wantSQL string, wantArgs []any) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != wantSQL {
		t.Fatalf("sql mismatch:\n got=%q\nwant=%q", sql, wantSQL)
	}
	if len(args) == 0 && len(wantArgs) == 0 {
		return
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args mismatch:\n got=%v\nwant=%v", args, wantArgs)
	}
}

// -----------------------------------------------------------------------------
// SELECT builder
// -----------------------------------------------------------------------------

func TestSelectSQLVariants(t *testing.T) {
	tests := []struct {
		name     string
		build    func(dialect.Renderer) (string, []any, error)
		wantSQL  string
		wantArgs []any
	}{
		{
			name: "all columns simple",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id", "email").Build()
			},
			wantSQL: `SELECT "id", "email" FROM "users"`,
		},
		{
			name: "distinct",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Distinct().Columns("country").Build()
			},
			wantSQL: `SELECT DISTINCT "country" FROM "users"`,
		},
		{
			name: "table alias",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Alias("u").Columns("id").Build()
			},
			wantSQL: `SELECT "id" FROM "users" AS "u"`,
		},
		{
			name: "where equality",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id").Where(query.Equal("id", 7)).Build()
			},
			wantSQL:  `SELECT "id" FROM "users" WHERE "id" = ?`,
			wantArgs: []any{7},
		},
		{
			name: "where comparison operators",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Columns("id").Where(query.And(
					query.GreaterThan("total", 100),
					query.LessThanOrEqual("total", 500),
					query.NotEqual("status", "void"),
				)).Build()
			},
			wantSQL:  `SELECT "id" FROM "orders" WHERE ("total" > ? AND "total" <= ? AND "status" <> ?)`,
			wantArgs: []any{100, 500, "void"},
		},
		{
			name: "where less/greater/equal boundaries",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "metrics").Columns("id").Where(query.And(
					query.LessThan("a", 1),
					query.GreaterThanOrEqual("b", 2),
				)).Build()
			},
			wantSQL:  `SELECT "id" FROM "metrics" WHERE ("a" < ? AND "b" >= ?)`,
			wantArgs: []any{1, 2},
		},
		{
			name: "nested and/or",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "records").Columns("id").Where(query.Or(
					query.And(query.Equal("a", 1), query.Equal("b", 2)),
					query.Equal("c", 3),
				)).Build()
			},
			wantSQL:  `SELECT "id" FROM "records" WHERE (("a" = ? AND "b" = ?) OR "c" = ?)`,
			wantArgs: []any{1, 2, 3},
		},
		{
			name: "in and not in",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "records").Columns("id").Where(query.And(
					query.In("status", "a", "b"),
					query.NotIn("kind", "x"),
				)).Build()
			},
			wantSQL:  `SELECT "id" FROM "records" WHERE ("status" IN (?, ?) AND "kind" NOT IN (?))`,
			wantArgs: []any{"a", "b", "x"},
		},
		{
			name: "between and not between",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "events").Columns("id").Where(query.And(
					query.Between("age", 18, 65),
					query.NotBetween("score", 0, 10),
				)).Build()
			},
			wantSQL:  `SELECT "id" FROM "events" WHERE ("age" BETWEEN ? AND ? AND "score" NOT BETWEEN ? AND ?)`,
			wantArgs: []any{18, 65, 0, 10},
		},
		{
			name: "like and not like",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id").Where(query.And(
					query.Like("email", "%@example.com"),
					query.NotLike("name", "test%"),
				)).Build()
			},
			wantSQL:  `SELECT "id" FROM "users" WHERE ("email" LIKE ? AND "name" NOT LIKE ?)`,
			wantArgs: []any{"%@example.com", "test%"},
		},
		{
			name: "is null and is not null",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id").Where(query.And(
					query.IsNull("deleted_at"),
					query.IsNotNull("verified_at"),
				)).Build()
			},
			wantSQL: `SELECT "id" FROM "users" WHERE ("deleted_at" IS NULL AND "verified_at" IS NOT NULL)`,
		},
		{
			name: "projections with aliases and functions",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Projections(
					query.Project(query.Column("country")),
					query.ProjectAs(query.Count(query.Column("id")), "total"),
					query.ProjectAs(query.Sum(query.Column("amount")), "revenue"),
				).GroupBy(query.Column("country")).Build()
			},
			wantSQL: `SELECT "country", COUNT("id") AS "total", SUM("amount") AS "revenue" FROM "orders" GROUP BY "country"`,
		},
		{
			name: "coalesce projection",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Projections(
					query.ProjectAs(query.Coalesce(query.Column("nickname"), query.Value("anon")), "display"),
				).Build()
			},
			wantSQL:  `SELECT COALESCE("nickname", ?) AS "display" FROM "orders"`,
			wantArgs: []any{"anon"},
		},
		{
			name: "arithmetic projection",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Projections(
					query.ProjectAs(query.Subtract(query.Column("gross"), query.Column("tax")), "net"),
				).Build()
			},
			wantSQL: `SELECT "gross" - "tax" AS "net" FROM "orders"`,
		},
		{
			name: "group by having",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Projections(
					query.Project(query.Column("country")),
					query.ProjectAs(query.Count(query.Column("id")), "total"),
				).GroupBy(query.Column("country")).Having(
					query.GreaterThanExpression(query.Count(query.Column("id")), 10),
				).Build()
			},
			wantSQL:  `SELECT "country", COUNT("id") AS "total" FROM "orders" GROUP BY "country" HAVING COUNT("id") > ?`,
			wantArgs: []any{10},
		},
		{
			name: "order by asc and desc",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id").OrderBy(
					query.Ascending("name"),
					query.Descending("created_at"),
				).Build()
			},
			wantSQL: `SELECT "id" FROM "users" ORDER BY "name" ASC, "created_at" DESC`,
		},
		{
			name: "limit only",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id").Limit(10).Build()
			},
			wantSQL:  `SELECT "id" FROM "users" LIMIT ?`,
			wantArgs: []any{10},
		},
		{
			name: "offset only",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "users").Columns("id").Offset(5).Build()
			},
			wantSQL:  `SELECT "id" FROM "users" OFFSET ?`,
			wantArgs: []any{5},
		},
		{
			name: "inner join",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Alias("o").Projections(
					query.Project(query.QualifiedColumn("o", "id")),
					query.Project(query.QualifiedColumn("u", "email")),
				).Join(
					query.InnerJoin("users", "u", query.EqualExpressions(
						query.QualifiedColumn("o", "user_id"),
						query.QualifiedColumn("u", "id"),
					)),
				).Build()
			},
			wantSQL: `SELECT "o"."id", "u"."email" FROM "orders" AS "o" INNER JOIN "users" AS "u" ON "o"."user_id" = "u"."id"`,
		},
		{
			name: "left join with where",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Alias("o").Columns("id").Join(
					query.LeftJoin("users", "u", query.EqualExpressions(
						query.QualifiedColumn("o", "user_id"),
						query.QualifiedColumn("u", "id"),
					)),
				).Where(query.IsNull("u")).Build()
			},
			wantSQL: `SELECT "id" FROM "orders" AS "o" LEFT JOIN "users" AS "u" ON "o"."user_id" = "u"."id" WHERE "u" IS NULL`,
		},
		{
			name: "full featured query",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Alias("o").Distinct().Projections(
					query.Project(query.QualifiedColumn("o", "country")),
					query.ProjectAs(query.Sum(query.QualifiedColumn("o", "amount")), "revenue"),
				).Join(
					query.InnerJoin("users", "u", query.EqualExpressions(
						query.QualifiedColumn("o", "user_id"),
						query.QualifiedColumn("u", "id"),
					)),
				).Where(query.And(
					query.EqualValue(query.QualifiedColumn("o", "status"), "paid"),
					query.InExpression(query.QualifiedColumn("o", "country"), "US", "CA"),
				)).GroupBy(query.QualifiedColumn("o", "country")).Having(
					query.GreaterThanExpression(query.Sum(query.QualifiedColumn("o", "amount")), 1000),
				).OrderBy(query.Descending("revenue")).Limit(20).Offset(40).Build()
			},
			wantSQL:  `SELECT DISTINCT "o"."country", SUM("o"."amount") AS "revenue" FROM "orders" AS "o" INNER JOIN "users" AS "u" ON "o"."user_id" = "u"."id" WHERE ("o"."status" = ? AND "o"."country" IN (?, ?)) GROUP BY "o"."country" HAVING SUM("o"."amount") > ? ORDER BY "revenue" DESC LIMIT ? OFFSET ?`,
			wantArgs: []any{"paid", "US", "CA", 1000, 20, 40},
		},
		{
			name: "in expression predicate",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "orders").Columns("id").Where(
					query.InExpression(query.QualifiedColumn("o", "status"), "paid", "shipped"),
				).Build()
			},
			wantSQL:  `SELECT "id" FROM "orders" WHERE "o"."status" IN (?, ?)`,
			wantArgs: []any{"paid", "shipped"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sql, args, err := test.build(sqliteRenderer(t))
			assertSQL(t, sql, args, err, test.wantSQL, test.wantArgs)
		})
	}
}

func TestSelectPostgresPlaceholders(t *testing.T) {
	sql, args, err := query.NewSelectBuilder(postgresRenderer(t), "users").Columns("id").Where(query.And(
		query.Equal("country", "US"),
		query.In("status", "a", "b"),
		query.Between("age", 18, 65),
	)).Limit(10).Build()
	assertSQL(t, sql, args, err,
		`SELECT "id" FROM "users" WHERE ("country" = $1 AND "status" IN ($2, $3) AND "age" BETWEEN $4 AND $5) LIMIT $6`,
		[]any{"US", "a", "b", 18, 65, 10})
}

// -----------------------------------------------------------------------------
// INSERT builder
// -----------------------------------------------------------------------------

func TestInsertSQLVariants(t *testing.T) {
	t.Run("single row", func(t *testing.T) {
		sql, args, err := query.NewInsertBuilder(sqliteRenderer(t), "users").
			Columns("id", "email").Values("u1", "a@b.co").Build()
		assertSQL(t, sql, args, err, `INSERT INTO "users" ("id", "email") VALUES (?, ?)`, []any{"u1", "a@b.co"})
	})
	t.Run("multi row", func(t *testing.T) {
		sql, args, err := query.NewInsertBuilder(sqliteRenderer(t), "users").
			Columns("id", "email").
			Values("u1", "a@b.co").
			Values("u2", "c@d.co").
			Values("u3", "e@f.co").Build()
		assertSQL(t, sql, args, err,
			`INSERT INTO "users" ("id", "email") VALUES (?, ?), (?, ?), (?, ?)`,
			[]any{"u1", "a@b.co", "u2", "c@d.co", "u3", "e@f.co"})
	})
	t.Run("null value", func(t *testing.T) {
		sql, args, err := query.NewInsertBuilder(sqliteRenderer(t), "users").
			Columns("id", "deleted_at").Values("u1", nil).Build()
		assertSQL(t, sql, args, err, `INSERT INTO "users" ("id", "deleted_at") VALUES (?, ?)`, []any{"u1", nil})
	})
	t.Run("postgres placeholders", func(t *testing.T) {
		sql, args, err := query.NewInsertBuilder(postgresRenderer(t), "users").
			Columns("id", "email").
			Values("u1", "a@b.co").
			Values("u2", "c@d.co").Build()
		assertSQL(t, sql, args, err,
			`INSERT INTO "users" ("id", "email") VALUES ($1, $2), ($3, $4)`,
			[]any{"u1", "a@b.co", "u2", "c@d.co"})
	})
}

// -----------------------------------------------------------------------------
// UPDATE builder
// -----------------------------------------------------------------------------

func TestUpdateSQLVariants(t *testing.T) {
	t.Run("single assignment", func(t *testing.T) {
		sql, args, err := query.NewUpdateBuilder(sqliteRenderer(t), "users").
			Set("email", "new@b.co").Where(query.Equal("id", "u1")).Build()
		assertSQL(t, sql, args, err, `UPDATE "users" SET "email" = ? WHERE "id" = ?`, []any{"new@b.co", "u1"})
	})
	t.Run("multiple assignments", func(t *testing.T) {
		sql, args, err := query.NewUpdateBuilder(sqliteRenderer(t), "users").
			Set("email", "new@b.co").
			Set("name", "Ann").
			Where(query.Equal("id", "u1")).Build()
		assertSQL(t, sql, args, err,
			`UPDATE "users" SET "email" = ?, "name" = ? WHERE "id" = ?`,
			[]any{"new@b.co", "Ann", "u1"})
	})
	t.Run("expression assignment increment", func(t *testing.T) {
		sql, args, err := query.NewUpdateBuilder(sqliteRenderer(t), "counters").
			SetExpression("hits", query.Add(query.Column("hits"), query.Value(1))).
			Where(query.Equal("id", "c1")).Build()
		assertSQL(t, sql, args, err,
			`UPDATE "counters" SET "hits" = "hits" + ? WHERE "id" = ?`,
			[]any{1, "c1"})
	})
	t.Run("expression assignment decrement", func(t *testing.T) {
		sql, args, err := query.NewUpdateBuilder(sqliteRenderer(t), "inventory").
			SetExpression("stock", query.Subtract(query.Column("stock"), query.Value(3))).
			Where(query.Equal("sku", "abc")).Build()
		assertSQL(t, sql, args, err,
			`UPDATE "inventory" SET "stock" = "stock" - ? WHERE "sku" = ?`,
			[]any{3, "abc"})
	})
	t.Run("mixed value and expression with complex predicate", func(t *testing.T) {
		sql, args, err := query.NewUpdateBuilder(sqliteRenderer(t), "records").
			Set("name", "Updated").
			SetExpression("revision", query.Add(query.Column("revision"), query.Value(1))).
			Where(query.And(query.Equal("workspace_id", "w1"), query.In("status", "a", "b"))).Build()
		assertSQL(t, sql, args, err,
			`UPDATE "records" SET "name" = ?, "revision" = "revision" + ? WHERE ("workspace_id" = ? AND "status" IN (?, ?))`,
			[]any{"Updated", 1, "w1", "a", "b"})
	})
	t.Run("postgres placeholders", func(t *testing.T) {
		sql, args, err := query.NewUpdateBuilder(postgresRenderer(t), "records").
			Set("name", "Updated").
			SetExpression("revision", query.Add(query.Column("revision"), query.Value(1))).
			Where(query.Equal("id", "one")).Build()
		assertSQL(t, sql, args, err,
			`UPDATE "records" SET "name" = $1, "revision" = "revision" + $2 WHERE "id" = $3`,
			[]any{"Updated", 1, "one"})
	})
}

// -----------------------------------------------------------------------------
// DELETE builder
// -----------------------------------------------------------------------------

func TestDeleteSQLVariants(t *testing.T) {
	t.Run("equality", func(t *testing.T) {
		sql, args, err := query.NewDeleteBuilder(sqliteRenderer(t), "users").
			Where(query.Equal("id", "u1")).Build()
		assertSQL(t, sql, args, err, `DELETE FROM "users" WHERE "id" = ?`, []any{"u1"})
	})
	t.Run("compound predicate", func(t *testing.T) {
		sql, args, err := query.NewDeleteBuilder(sqliteRenderer(t), "sessions").
			Where(query.And(query.LessThan("expires_at", 1000), query.IsNotNull("user_id"))).Build()
		assertSQL(t, sql, args, err,
			`DELETE FROM "sessions" WHERE ("expires_at" < ? AND "user_id" IS NOT NULL)`,
			[]any{1000})
	})
	t.Run("in predicate", func(t *testing.T) {
		sql, args, err := query.NewDeleteBuilder(sqliteRenderer(t), "records").
			Where(query.In("id", "a", "b", "c")).Build()
		assertSQL(t, sql, args, err, `DELETE FROM "records" WHERE "id" IN (?, ?, ?)`, []any{"a", "b", "c"})
	})
	t.Run("postgres placeholders", func(t *testing.T) {
		sql, args, err := query.NewDeleteBuilder(postgresRenderer(t), "records").
			Where(query.And(query.Equal("workspace_id", "w1"), query.Equal("id", "one"))).Build()
		assertSQL(t, sql, args, err,
			`DELETE FROM "records" WHERE ("workspace_id" = $1 AND "id" = $2)`,
			[]any{"w1", "one"})
	})
}

// -----------------------------------------------------------------------------
// Namespace / schema qualification
// -----------------------------------------------------------------------------

func TestBuildersHonorNamespace(t *testing.T) {
	renderer, err := dialect.ParseRenderer("postgres", "app", "t_")
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	selectSQL, _, err := query.NewSelectBuilder(renderer, "users").Columns("id").Build()
	assertSQL(t, selectSQL, nil, err, `SELECT "id" FROM "app"."t_users"`, nil)

	insertSQL, insertArgs, err := query.NewInsertBuilder(renderer, "users").Columns("id").Values("u1").Build()
	assertSQL(t, insertSQL, insertArgs, err, `INSERT INTO "app"."t_users" ("id") VALUES ($1)`, []any{"u1"})

	updateSQL, updateArgs, err := query.NewUpdateBuilder(renderer, "users").Set("id", "u2").Where(query.Equal("id", "u1")).Build()
	assertSQL(t, updateSQL, updateArgs, err, `UPDATE "app"."t_users" SET "id" = $1 WHERE "id" = $2`, []any{"u2", "u1"})

	deleteSQL, deleteArgs, err := query.NewDeleteBuilder(renderer, "users").Where(query.Equal("id", "u1")).Build()
	assertSQL(t, deleteSQL, deleteArgs, err, `DELETE FROM "app"."t_users" WHERE "id" = $1`, []any{"u1"})
}

// -----------------------------------------------------------------------------
// Error / validation coverage
// -----------------------------------------------------------------------------

func TestBuilderErrors(t *testing.T) {
	r := sqliteRenderer(t)
	cases := []struct {
		name  string
		build func() (string, []any, error)
	}{
		{"select without columns", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Build()
		}},
		{"select empty table", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "   ").Columns("id").Build()
		}},
		{"select negative limit", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Columns("id").Limit(-1).Build()
		}},
		{"select negative offset", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Columns("id").Offset(-1).Build()
		}},
		{"select empty in", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Columns("id").Where(query.In("status")).Build()
		}},
		{"select empty compound", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Columns("id").Where(query.And()).Build()
		}},
		{"select nil group expression", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Columns("id").GroupBy(nil).Build()
		}},
		{"select join without predicate", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "orders").Columns("id").Join(query.InnerJoin("users", "u", nil)).Build()
		}},
		{"projection nil expression", func() (string, []any, error) {
			return query.NewSelectBuilder(r, "users").Projections(query.Project(nil)).Build()
		}},
		{"insert without columns", func() (string, []any, error) {
			return query.NewInsertBuilder(r, "users").Values("u1").Build()
		}},
		{"insert without values", func() (string, []any, error) {
			return query.NewInsertBuilder(r, "users").Columns("id").Build()
		}},
		{"insert row arity mismatch", func() (string, []any, error) {
			return query.NewInsertBuilder(r, "users").Columns("id", "email").Values("only-id").Build()
		}},
		{"update without assignments", func() (string, []any, error) {
			return query.NewUpdateBuilder(r, "users").Where(query.Equal("id", "u1")).Build()
		}},
		{"update without predicate", func() (string, []any, error) {
			return query.NewUpdateBuilder(r, "users").Set("name", "x").Build()
		}},
		{"update blank column", func() (string, []any, error) {
			return query.NewUpdateBuilder(r, "users").Set("  ", "x").Where(query.Equal("id", "u1")).Build()
		}},
		{"delete without predicate", func() (string, []any, error) {
			return query.NewDeleteBuilder(r, "users").Build()
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := test.build(); err == nil {
				t.Fatalf("expected error for %s", test.name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Full API surface coverage: every remaining constructor not exercised above.
// -----------------------------------------------------------------------------

func TestExpressionCoverage(t *testing.T) {
	tests := []struct {
		name     string
		build    func(dialect.Renderer) (string, []any, error)
		wantSQL  string
		wantArgs []any
	}{
		{
			name: "count all",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "t").Projections(query.ProjectAs(query.CountAll(), "n")).Build()
			},
			wantSQL: `SELECT COUNT(*) AS "n" FROM "t"`,
		},
		{
			name: "count column",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "t").Projections(query.Project(query.Count(query.Column("id")))).Build()
			},
			wantSQL: `SELECT COUNT("id") FROM "t"`,
		},
		{
			name: "max and min",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "t").Projections(
					query.ProjectAs(query.Max(query.Column("price")), "hi"),
					query.ProjectAs(query.Min(query.Column("price")), "lo"),
				).Build()
			},
			wantSQL: `SELECT MAX("price") AS "hi", MIN("price") AS "lo" FROM "t"`,
		},
		{
			name: "lower",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "t").Projections(query.Project(query.Lower(query.Column("name")))).Build()
			},
			wantSQL: `SELECT LOWER("name") FROM "t"`,
		},
		{
			name: "add and subtract nested",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "t").Projections(
					query.ProjectAs(query.Add(query.Subtract(query.Column("a"), query.Column("b")), query.Value(1)), "x"),
				).Build()
			},
			wantSQL:  `SELECT "a" - "b" + ? AS "x" FROM "t"`,
			wantArgs: []any{1},
		},
		{
			name: "case with multiple when branches",
			build: func(r dialect.Renderer) (string, []any, error) {
				return query.NewSelectBuilder(r, "t").Projections(
					query.ProjectAs(query.CaseWhen(query.Equal("grade", "A"), 4).
						When(query.Equal("grade", "B"), 3).
						When(query.Equal("grade", "C"), 2).
						Else(0), "gpa"),
				).Build()
			},
			wantSQL:  `SELECT CASE WHEN "grade" = ? THEN ? WHEN "grade" = ? THEN ? WHEN "grade" = ? THEN ? ELSE ? END AS "gpa" FROM "t"`,
			wantArgs: []any{"A", 4, "B", 3, "C", 2, 0},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sql, args, err := test.build(sqliteRenderer(t))
			assertSQL(t, sql, args, err, test.wantSQL, test.wantArgs)
		})
	}
}

func TestPredicateExpressionComparisonCoverage(t *testing.T) {
	col := func(c string) query.Expression { return query.QualifiedColumn("t", c) }
	tests := []struct {
		name      string
		predicate query.Predicate
		wantSQL   string
		wantArgs  []any
	}{
		{"not equal expressions", query.NotEqualExpressions(col("a"), col("b")), `"t"."a" <> "t"."b"`, nil},
		{"less than expressions", query.LessThanExpressions(col("a"), col("b")), `"t"."a" < "t"."b"`, nil},
		{"less or equal expressions", query.LessThanOrEqualExpressions(col("a"), col("b")), `"t"."a" <= "t"."b"`, nil},
		{"greater than expressions", query.GreaterThanExpressions(col("a"), col("b")), `"t"."a" > "t"."b"`, nil},
		{"greater or equal expressions", query.GreaterThanOrEqualExpressions(col("a"), col("b")), `"t"."a" >= "t"."b"`, nil},
		{"not equal value", query.NotEqualValue(col("a"), 1), `"t"."a" <> ?`, []any{1}},
		{"less than value", query.LessThanValue(col("a"), 1), `"t"."a" < ?`, []any{1}},
		{"less or equal value", query.LessThanOrEqualValue(col("a"), 1), `"t"."a" <= ?`, []any{1}},
		{"greater than value", query.GreaterThanExpression(col("a"), 1), `"t"."a" > ?`, []any{1}},
		{"greater or equal value", query.GreaterThanOrEqualValue(col("a"), 1), `"t"."a" >= ?`, []any{1}},
		{"like value", query.LikeValue(col("a"), "x%"), `"t"."a" LIKE ?`, []any{"x%"}},
		{"not like value", query.NotLikeValue(col("a"), "x%"), `"t"."a" NOT LIKE ?`, []any{"x%"}},
		{"not in expression", query.NotInExpression(col("a"), 1, 2), `"t"."a" NOT IN (?, ?)`, []any{1, 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sql, args, err := query.NewSelectBuilder(sqliteRenderer(t), "t").Columns("id").Where(test.predicate).Build()
			assertSQL(t, sql, args, err, `SELECT "id" FROM "t" WHERE `+test.wantSQL, test.wantArgs)
		})
	}
}

func TestOrderByExpressionCoverage(t *testing.T) {
	t.Run("ascending expression", func(t *testing.T) {
		sql, args, err := query.NewSelectBuilder(sqliteRenderer(t), "t").Columns("id").
			OrderBy(query.AscendingExpression(query.Lower(query.Column("name")))).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "t" ORDER BY LOWER("name") ASC`, nil)
	})
	t.Run("descending expression", func(t *testing.T) {
		sql, args, err := query.NewSelectBuilder(sqliteRenderer(t), "t").Columns("id").
			OrderBy(query.DescendingExpression(query.Count(query.Column("id")))).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "t" ORDER BY COUNT("id") DESC`, nil)
	})
	t.Run("mixed column and expression order", func(t *testing.T) {
		sql, args, err := query.NewSelectBuilder(sqliteRenderer(t), "t").Columns("id").OrderBy(
			query.Ascending("a"),
			query.DescendingExpression(query.Lower(query.Column("b"))),
		).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "t" ORDER BY "a" ASC, LOWER("b") DESC`, nil)
	})
}

func TestExpressionLikeInHaving(t *testing.T) {
	sql, args, err := query.NewSelectBuilder(sqliteRenderer(t), "t").
		Projections(query.Project(query.Column("bucket")), query.ProjectAs(query.Count(query.Column("id")), "n")).
		GroupBy(query.Column("bucket")).
		Having(query.And(
			query.GreaterThanExpression(query.Count(query.Column("id")), 5),
			query.LikeValue(query.Lower(query.Column("bucket")), "a%"),
		)).Build()
	assertSQL(t, sql, args, err,
		`SELECT "bucket", COUNT("id") AS "n" FROM "t" GROUP BY "bucket" HAVING (COUNT("id") > ? AND LOWER("bucket") LIKE ?)`,
		[]any{5, "a%"})
}
