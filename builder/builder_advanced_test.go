package builder_test

import (
	"testing"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
)

func mysqlRenderer(t *testing.T) dialect.Renderer {
	t.Helper()
	renderer, err := dialect.ParseRenderer("mysql", "", "")
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return renderer
}

// -----------------------------------------------------------------------------
// Row locking: FOR UPDATE / FOR SHARE (+ NOWAIT / SKIP LOCKED)
// -----------------------------------------------------------------------------

func TestSelectRowLocking(t *testing.T) {
	tests := []struct {
		name     string
		build    func(dialect.Renderer) (string, []any, error)
		wantSQL  string
		wantArgs []any
	}{
		{"for update", func(r dialect.Renderer) (string, []any, error) {
			return builder.NewSelectBuilder(r, "jobs").Columns("id").Where(builder.Equal("state", "queued")).ForUpdate().Build()
		}, `SELECT "id" FROM "jobs" WHERE "state" = ? FOR UPDATE`, []any{"queued"}},
		{"for update skip locked", func(r dialect.Renderer) (string, []any, error) {
			return builder.NewSelectBuilder(r, "jobs").Columns("id").Where(builder.Equal("state", "queued")).ForUpdate("SKIP LOCKED").Build()
		}, `SELECT "id" FROM "jobs" WHERE "state" = ? FOR UPDATE SKIP LOCKED`, []any{"queued"}},
		{"for update nowait", func(r dialect.Renderer) (string, []any, error) {
			return builder.NewSelectBuilder(r, "jobs").Columns("id").Where(builder.Equal("id", 1)).ForUpdate("NOWAIT").Build()
		}, `SELECT "id" FROM "jobs" WHERE "id" = ? FOR UPDATE NOWAIT`, []any{1}},
		{"for share", func(r dialect.Renderer) (string, []any, error) {
			return builder.NewSelectBuilder(r, "jobs").Columns("id").Where(builder.Equal("id", 1)).ForShare().Build()
		}, `SELECT "id" FROM "jobs" WHERE "id" = ? FOR SHARE`, []any{1}},
		{"for no key update", func(r dialect.Renderer) (string, []any, error) {
			return builder.NewSelectBuilder(r, "jobs").Columns("id").Where(builder.Equal("id", 1)).ForNoKeyUpdate().Build()
		}, `SELECT "id" FROM "jobs" WHERE "id" = ? FOR NO KEY UPDATE`, []any{1}},
		{"for key share", func(r dialect.Renderer) (string, []any, error) {
			return builder.NewSelectBuilder(r, "jobs").Columns("id").Where(builder.Equal("id", 1)).ForKeyShare().Build()
		}, `SELECT "id" FROM "jobs" WHERE "id" = ? FOR KEY SHARE`, []any{1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sql, args, err := test.build(sqliteRenderer(t))
			assertSQL(t, sql, args, err, test.wantSQL, test.wantArgs)
		})
	}
}

// -----------------------------------------------------------------------------
// Set operations: UNION / INTERSECT / EXCEPT (+ ALL)
// -----------------------------------------------------------------------------

func TestSelectSetOperations(t *testing.T) {
	base := func(r dialect.Renderer, table string, value any) *builder.SelectBuilder {
		return builder.NewSelectBuilder(r, table).Columns("id").Where(builder.Equal("kind", value))
	}
	t.Run("union", func(t *testing.T) {
		q := base(sqliteRenderer(t), "a", "x").Union(base(sqliteRenderer(t), "b", "y"))
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "a" WHERE "kind" = ? UNION SELECT "id" FROM "b" WHERE "kind" = ?`,
			[]any{"x", "y"})
	})
	t.Run("union all with order limit", func(t *testing.T) {
		q := base(sqliteRenderer(t), "a", "x").
			UnionAll(base(sqliteRenderer(t), "b", "y")).
			OrderBy(builder.Ascending("id")).Limit(10)
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "a" WHERE "kind" = ? UNION ALL SELECT "id" FROM "b" WHERE "kind" = ? ORDER BY "id" ASC LIMIT ?`,
			[]any{"x", "y", 10})
	})
	t.Run("intersect and except", func(t *testing.T) {
		q := base(sqliteRenderer(t), "a", "x").
			Intersect(base(sqliteRenderer(t), "b", "y")).
			Except(base(sqliteRenderer(t), "c", "z"))
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "a" WHERE "kind" = ? INTERSECT SELECT "id" FROM "b" WHERE "kind" = ? EXCEPT SELECT "id" FROM "c" WHERE "kind" = ?`,
			[]any{"x", "y", "z"})
	})
	t.Run("postgres placeholder numbering across union", func(t *testing.T) {
		q := base(postgresRenderer(t), "a", "x").Union(base(postgresRenderer(t), "b", "y"))
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "a" WHERE "kind" = $1 UNION SELECT "id" FROM "b" WHERE "kind" = $2`,
			[]any{"x", "y"})
	})
}

// -----------------------------------------------------------------------------
// CTEs: WITH / WITH RECURSIVE
// -----------------------------------------------------------------------------

func TestSelectCTE(t *testing.T) {
	t.Run("single cte", func(t *testing.T) {
		active := builder.NewSelectBuilder(sqliteRenderer(t), "users").Columns("id").Where(builder.Equal("active", true))
		q := builder.NewSelectBuilder(sqliteRenderer(t), "active_users").Columns("id").With("active_users", active)
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`WITH "active_users" AS (SELECT "id" FROM "users" WHERE "active" = ?) SELECT "id" FROM "active_users"`,
			[]any{true})
	})
	t.Run("cte with column list", func(t *testing.T) {
		source := builder.NewSelectBuilder(sqliteRenderer(t), "orders").
			Projections(builder.Project(builder.Column("user_id")), builder.ProjectAs(builder.Count(builder.Column("id")), "n")).
			GroupBy(builder.Column("user_id"))
		q := builder.NewSelectBuilder(sqliteRenderer(t), "totals").Columns("user_id", "n").With("totals", source, "user_id", "n")
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`WITH "totals" ("user_id", "n") AS (SELECT "user_id", COUNT("id") AS "n" FROM "orders" GROUP BY "user_id") SELECT "user_id", "n" FROM "totals"`,
			nil)
	})
	t.Run("recursive cte", func(t *testing.T) {
		seed := builder.NewSelectBuilder(sqliteRenderer(t), "tree").Columns("id", "parent_id").Where(builder.IsNull("parent_id"))
		q := builder.NewSelectBuilder(sqliteRenderer(t), "descendants").Columns("id").WithRecursive("descendants", seed)
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`WITH RECURSIVE "descendants" AS (SELECT "id", "parent_id" FROM "tree" WHERE "parent_id" IS NULL) SELECT "id" FROM "descendants"`,
			nil)
	})
}

// -----------------------------------------------------------------------------
// Derived tables (FROM subquery) and LATERAL
// -----------------------------------------------------------------------------

func TestSelectDerivedTable(t *testing.T) {
	t.Run("from subquery", func(t *testing.T) {
		inner := builder.NewSelectBuilder(sqliteRenderer(t), "events").
			Projections(builder.Project(builder.Column("user_id")), builder.ProjectAs(builder.Count(builder.Column("id")), "n")).
			GroupBy(builder.Column("user_id"))
		q := builder.NewSelectFromSubquery(sqliteRenderer(t), inner, "e").
			Projections(builder.Project(builder.QualifiedColumn("e", "user_id"))).
			Where(builder.GreaterThanExpression(builder.QualifiedColumn("e", "n"), 5))
		sql, args, err := q.Build()
		assertSQL(t, sql, args, err,
			`SELECT "e"."user_id" FROM (SELECT "user_id", COUNT("id") AS "n" FROM "events" GROUP BY "user_id") AS "e" WHERE "e"."n" > ?`,
			[]any{5})
	})
	t.Run("lateral subquery join", func(t *testing.T) {
		lateral := builder.NewSelectBuilder(postgresRenderer(t), "orders").Columns("id").
			Where(builder.EqualExpressions(builder.QualifiedColumn("orders", "user_id"), builder.QualifiedColumn("u", "id"))).
			Limit(1)
		on := builder.EqualExpressions(builder.QualifiedColumn("o", "id"), builder.QualifiedColumn("o", "id"))
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "users").Alias("u").
			Projections(builder.Project(builder.QualifiedColumn("u", "id"))).
			Join(builder.LeftJoinSubquery(lateral, "o", on).Lateral()).Build()
		assertSQL(t, sql, args, err,
			`SELECT "u"."id" FROM "users" AS "u" LEFT JOIN LATERAL (SELECT "id" FROM "orders" WHERE "orders"."user_id" = "u"."id" LIMIT $1) AS "o" ON "o"."id" = "o"."id"`,
			[]any{1})
	})
}

// -----------------------------------------------------------------------------
// DISTINCT ON, ROLLUP / CUBE / GROUPING SETS
// -----------------------------------------------------------------------------

func TestSelectGroupingAndDistinctOn(t *testing.T) {
	t.Run("distinct on", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "events").
			DistinctOn(builder.Column("user_id")).
			Projections(builder.Project(builder.Column("user_id")), builder.Project(builder.Column("created_at"))).
			OrderBy(builder.Ascending("user_id"), builder.Descending("created_at")).Build()
		assertSQL(t, sql, args, err,
			`SELECT DISTINCT ON ("user_id") "user_id", "created_at" FROM "events" ORDER BY "user_id" ASC, "created_at" DESC`,
			nil)
	})
	t.Run("rollup", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "sales").
			Projections(builder.Project(builder.Column("region")), builder.ProjectAs(builder.Sum(builder.Column("amount")), "total")).
			GroupByRollup(builder.Column("region"), builder.Column("product")).Build()
		assertSQL(t, sql, args, err,
			`SELECT "region", SUM("amount") AS "total" FROM "sales" GROUP BY ROLLUP ("region", "product")`,
			nil)
	})
	t.Run("cube", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "sales").
			Projections(builder.ProjectAs(builder.Sum(builder.Column("amount")), "total")).
			GroupByCube(builder.Column("region")).Build()
		assertSQL(t, sql, args, err, `SELECT SUM("amount") AS "total" FROM "sales" GROUP BY CUBE ("region")`, nil)
	})
	t.Run("grouping sets", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "sales").
			Projections(builder.ProjectAs(builder.Sum(builder.Column("amount")), "total")).
			GroupByGroupingSets(builder.Column("region"), builder.Column("product")).Build()
		assertSQL(t, sql, args, err,
			`SELECT SUM("amount") AS "total" FROM "sales" GROUP BY GROUPING SETS ("region", "product")`, nil)
	})
}

// -----------------------------------------------------------------------------
// Joins: RIGHT / FULL / CROSS
// -----------------------------------------------------------------------------

func TestJoinVariants(t *testing.T) {
	on := builder.EqualExpressions(builder.QualifiedColumn("a", "id"), builder.QualifiedColumn("b", "a_id"))
	t.Run("right join", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "a").Alias("a").Columns("id").
			Join(builder.RightJoin("b", "b", on)).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "a" AS "a" RIGHT JOIN "b" AS "b" ON "a"."id" = "b"."a_id"`, nil)
	})
	t.Run("full join", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "a").Alias("a").Columns("id").
			Join(builder.FullJoin("b", "b", on)).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "a" AS "a" FULL JOIN "b" AS "b" ON "a"."id" = "b"."a_id"`, nil)
	})
	t.Run("cross join", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "a").Alias("a").Columns("id").
			Join(builder.CrossJoin("b", "b")).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "a" AS "a" CROSS JOIN "b" AS "b"`, nil)
	})
	t.Run("subquery join", func(t *testing.T) {
		sub := builder.NewSelectBuilder(sqliteRenderer(t), "orders").
			Projections(builder.Project(builder.Column("user_id")), builder.ProjectAs(builder.Count(builder.Column("id")), "n")).
			GroupBy(builder.Column("user_id"))
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "users").Alias("u").
			Projections(builder.Project(builder.QualifiedColumn("u", "id"))).
			Join(builder.InnerJoinSubquery(sub, "o", builder.EqualExpressions(builder.QualifiedColumn("u", "id"), builder.QualifiedColumn("o", "user_id")))).Build()
		assertSQL(t, sql, args, err,
			`SELECT "u"."id" FROM "users" AS "u" INNER JOIN (SELECT "user_id", COUNT("id") AS "n" FROM "orders" GROUP BY "user_id") AS "o" ON "u"."id" = "o"."user_id"`,
			nil)
	})
}

// -----------------------------------------------------------------------------
// Subquery predicates: IN / comparison / EXISTS
// -----------------------------------------------------------------------------

func TestSubqueryPredicates(t *testing.T) {
	t.Run("in subquery", func(t *testing.T) {
		sub := builder.NewSelectBuilder(sqliteRenderer(t), "bans").Columns("user_id").Where(builder.Equal("active", true))
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "users").Columns("id").
			Where(builder.InSubquery("id", sub)).Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "users" WHERE "id" IN (SELECT "user_id" FROM "bans" WHERE "active" = ?)`,
			[]any{true})
	})
	t.Run("not in subquery", func(t *testing.T) {
		sub := builder.NewSelectBuilder(sqliteRenderer(t), "bans").Columns("user_id").Where(builder.Equal("active", true))
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "users").Columns("id").
			Where(builder.NotInSubquery("id", sub)).Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "users" WHERE "id" NOT IN (SELECT "user_id" FROM "bans" WHERE "active" = ?)`,
			[]any{true})
	})
	t.Run("scalar comparison subquery", func(t *testing.T) {
		avg := builder.NewSelectBuilder(sqliteRenderer(t), "orders").Projections(builder.Project(builder.Avg(builder.Column("total"))))
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "orders").Columns("id").
			Where(builder.GreaterThanSubquery(builder.Column("total"), avg)).Build()
		assertSQL(t, sql, args, err,
			`SELECT "id" FROM "orders" WHERE "total" > (SELECT AVG("total") FROM "orders")`,
			nil)
	})
	t.Run("exists subquery", func(t *testing.T) {
		sub := builder.NewSelectBuilder(sqliteRenderer(t), "orders").Columns("id").
			Where(builder.EqualExpressions(builder.QualifiedColumn("orders", "user_id"), builder.QualifiedColumn("u", "id")))
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "users").Alias("u").
			Projections(builder.Project(builder.QualifiedColumn("u", "id"))).
			Where(builder.ExistsSubquery(sub)).Build()
		assertSQL(t, sql, args, err,
			`SELECT "u"."id" FROM "users" AS "u" WHERE EXISTS (SELECT "id" FROM "orders" WHERE "orders"."user_id" = "u"."id")`,
			nil)
	})
	t.Run("not exists subquery", func(t *testing.T) {
		sub := builder.NewSelectBuilder(sqliteRenderer(t), "orders").Columns("id").
			Where(builder.EqualExpressions(builder.QualifiedColumn("orders", "user_id"), builder.QualifiedColumn("u", "id")))
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "users").Alias("u").
			Projections(builder.Project(builder.QualifiedColumn("u", "id"))).
			Where(builder.NotExistsSubquery(sub)).Build()
		assertSQL(t, sql, args, err,
			`SELECT "u"."id" FROM "users" AS "u" WHERE NOT EXISTS (SELECT "id" FROM "orders" WHERE "orders"."user_id" = "u"."id")`,
			nil)
	})
}

// -----------------------------------------------------------------------------
// ILIKE (dialect-aware) and NULLS ordering (dialect-aware)
// -----------------------------------------------------------------------------

func TestILikeDialects(t *testing.T) {
	t.Run("postgres native ilike", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "users").Columns("id").
			Where(builder.ILike("email", "%@EXAMPLE.com")).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "users" WHERE "email" ILIKE $1`, []any{"%@EXAMPLE.com"})
	})
	t.Run("mysql emulated ilike", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(mysqlRenderer(t), "users").Columns("id").
			Where(builder.ILike("email", "%@EXAMPLE.com")).Build()
		assertSQL(t, sql, args, err, "SELECT `id` FROM `users` WHERE LOWER(`email`) LIKE LOWER(?)", []any{"%@EXAMPLE.com"})
	})
	t.Run("sqlite emulated not ilike", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "users").Columns("id").
			Where(builder.NotILike("name", "admin%")).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "users" WHERE LOWER("name") NOT LIKE LOWER(?)`, []any{"admin%"})
	})
}

func TestNullsOrdering(t *testing.T) {
	t.Run("postgres nulls last", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "t").Columns("id").
			OrderBy(builder.Ascending("score").NullsLast()).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "t" ORDER BY "score" ASC NULLS LAST`, nil)
	})
	t.Run("sqlite nulls first", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "t").Columns("id").
			OrderBy(builder.Descending("score").NullsFirst()).Build()
		assertSQL(t, sql, args, err, `SELECT "id" FROM "t" ORDER BY "score" DESC NULLS FIRST`, nil)
	})
	t.Run("mysql emulated nulls last", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(mysqlRenderer(t), "t").Columns("id").
			OrderBy(builder.Ascending("score").NullsLast()).Build()
		assertSQL(t, sql, args, err, "SELECT `id` FROM `t` ORDER BY ISNULL(`score`) ASC, `score` ASC", nil)
	})
	t.Run("mysql emulated nulls first", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(mysqlRenderer(t), "t").Columns("id").
			OrderBy(builder.Descending("score").NullsFirst()).Build()
		assertSQL(t, sql, args, err, "SELECT `id` FROM `t` ORDER BY ISNULL(`score`) DESC, `score` DESC", nil)
	})
}

// -----------------------------------------------------------------------------
// Window functions, CAST, FILTER, Func, arithmetic
// -----------------------------------------------------------------------------

func TestWindowAndScalarExpressions(t *testing.T) {
	t.Run("row_number over partition order", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "events").Projections(
			builder.Project(builder.Column("id")),
			builder.ProjectAs(builder.Over(builder.RowNumber(), builder.Window().
				PartitionBy(builder.Column("user_id")).
				OrderBy(builder.Descending("created_at"))), "rn"),
		).Build()
		assertSQL(t, sql, args, err,
			`SELECT "id", ROW_NUMBER() OVER (PARTITION BY "user_id" ORDER BY "created_at" DESC) AS "rn" FROM "events"`,
			nil)
	})
	t.Run("sum over with frame", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "ledger").Projections(
			builder.ProjectAs(builder.Over(builder.Sum(builder.Column("amount")), builder.Window().
				OrderBy(builder.Ascending("day")).
				Frame("ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW")), "running"),
		).Build()
		assertSQL(t, sql, args, err,
			`SELECT SUM("amount") OVER (ORDER BY "day" ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS "running" FROM "ledger"`,
			nil)
	})
	t.Run("cast", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "t").Projections(
			builder.ProjectAs(builder.Cast(builder.Column("created_at"), "date"), "d"),
		).Build()
		assertSQL(t, sql, args, err, `SELECT CAST("created_at" AS date) AS "d" FROM "t"`, nil)
	})
	t.Run("filtered aggregate", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(postgresRenderer(t), "orders").Projections(
			builder.ProjectAs(builder.Filtered(builder.CountAll(), builder.Equal("status", "paid")), "paid_count"),
		).Build()
		assertSQL(t, sql, args, err,
			`SELECT COUNT(*) FILTER (WHERE "status" = $1) AS "paid_count" FROM "orders"`,
			[]any{"paid"})
	})
	t.Run("generic func and arithmetic", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "t").Projections(
			builder.ProjectAs(builder.Func("ROUND", builder.Divide(builder.Column("a"), builder.Column("b")), builder.Value(2)), "r"),
			builder.ProjectAs(builder.Multiply(builder.Column("x"), builder.Value(10)), "scaled"),
		).Build()
		assertSQL(t, sql, args, err,
			`SELECT ROUND("a" / "b", ?) AS "r", "x" * ? AS "scaled" FROM "t"`,
			[]any{2, 10})
	})
	t.Run("star projection", func(t *testing.T) {
		sql, args, err := builder.NewSelectBuilder(sqliteRenderer(t), "t").Projections(builder.Project(builder.Star())).Build()
		assertSQL(t, sql, args, err, `SELECT * FROM "t"`, nil)
	})
}

// -----------------------------------------------------------------------------
// RETURNING (INSERT / UPDATE / DELETE)
// -----------------------------------------------------------------------------

func TestReturningClauses(t *testing.T) {
	t.Run("insert returning", func(t *testing.T) {
		sql, args, err := builder.NewInsertBuilder(postgresRenderer(t), "users").
			Columns("email").Values("a@b.co").Returning("id", "created_at").Build()
		assertSQL(t, sql, args, err,
			`INSERT INTO "users" ("email") VALUES ($1) RETURNING "id", "created_at"`,
			[]any{"a@b.co"})
	})
	t.Run("update returning star", func(t *testing.T) {
		sql, args, err := builder.NewUpdateBuilder(postgresRenderer(t), "users").
			Set("name", "Ann").Where(builder.Equal("id", "u1")).Returning("*").Build()
		assertSQL(t, sql, args, err,
			`UPDATE "users" SET "name" = $1 WHERE "id" = $2 RETURNING *`,
			[]any{"Ann", "u1"})
	})
	t.Run("delete returning", func(t *testing.T) {
		sql, args, err := builder.NewDeleteBuilder(postgresRenderer(t), "sessions").
			Where(builder.Equal("id", "s1")).Returning("id").Build()
		assertSQL(t, sql, args, err, `DELETE FROM "sessions" WHERE "id" = $1 RETURNING "id"`, []any{"s1"})
	})
}

// -----------------------------------------------------------------------------
// Upsert: ON CONFLICT (PG/SQLite) and ON DUPLICATE KEY UPDATE (MySQL)
// -----------------------------------------------------------------------------

func TestUpsert(t *testing.T) {
	t.Run("on conflict do nothing", func(t *testing.T) {
		sql, args, err := builder.NewInsertBuilder(postgresRenderer(t), "users").
			Columns("id", "email").Values("u1", "a@b.co").
			OnConflictDoNothing("id").Build()
		assertSQL(t, sql, args, err,
			`INSERT INTO "users" ("id", "email") VALUES ($1, $2) ON CONFLICT ("id") DO NOTHING`,
			[]any{"u1", "a@b.co"})
	})
	t.Run("on conflict do update", func(t *testing.T) {
		sql, args, err := builder.NewInsertBuilder(postgresRenderer(t), "users").
			Columns("id", "email", "hits").Values("u1", "a@b.co", 1).
			OnConflictDoUpdate([]string{"id"},
				builder.Assign("email", "a@b.co"),
				builder.AssignExpression("hits", builder.Add(builder.Column("hits"), builder.Value(1))),
			).Build()
		assertSQL(t, sql, args, err,
			`INSERT INTO "users" ("id", "email", "hits") VALUES ($1, $2, $3) ON CONFLICT ("id") DO UPDATE SET "email" = $4, "hits" = "hits" + $5`,
			[]any{"u1", "a@b.co", 1, "a@b.co", 1})
	})
	t.Run("on conflict do update where", func(t *testing.T) {
		sql, args, err := builder.NewInsertBuilder(postgresRenderer(t), "counters").
			Columns("id", "hits").Values("c1", 1).
			OnConflictDoUpdateWhere([]string{"id"}, builder.GreaterThanExpression(builder.Column("hits"), 0),
				builder.AssignExpression("hits", builder.Add(builder.Column("hits"), builder.Value(1))),
			).Build()
		assertSQL(t, sql, args, err,
			`INSERT INTO "counters" ("id", "hits") VALUES ($1, $2) ON CONFLICT ("id") DO UPDATE SET "hits" = "hits" + $3 WHERE "hits" > $4`,
			[]any{"c1", 1, 1, 0})
	})
	t.Run("mysql on duplicate key update", func(t *testing.T) {
		sql, args, err := builder.NewInsertBuilder(mysqlRenderer(t), "users").
			Columns("id", "email").Values("u1", "a@b.co").
			OnDuplicateKeyUpdate(builder.Assign("email", "a@b.co")).Build()
		assertSQL(t, sql, args, err,
			"INSERT INTO `users` (`id`, `email`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `email` = ?",
			[]any{"u1", "a@b.co", "a@b.co"})
	})
}

// -----------------------------------------------------------------------------
// INSERT ... SELECT
// -----------------------------------------------------------------------------

func TestInsertFromSelect(t *testing.T) {
	source := builder.NewSelectBuilder(postgresRenderer(t), "staging_users").
		Columns("id", "email").Where(builder.Equal("valid", true))
	sql, args, err := builder.NewInsertBuilder(postgresRenderer(t), "users").
		Columns("id", "email").FromSelect(source).Build()
	assertSQL(t, sql, args, err,
		`INSERT INTO "users" ("id", "email") SELECT "id", "email" FROM "staging_users" WHERE "valid" = $1`,
		[]any{true})
}

// -----------------------------------------------------------------------------
// Error coverage for the new surface
// -----------------------------------------------------------------------------

func TestAdvancedErrors(t *testing.T) {
	r := sqliteRenderer(t)
	cases := []struct {
		name  string
		build func() (string, []any, error)
	}{
		{"subquery from without alias", func() (string, []any, error) {
			inner := builder.NewSelectBuilder(r, "t").Columns("id")
			return builder.NewSelectFromSubquery(r, inner, "").Columns("id").Build()
		}},
		{"cross join missing table", func() (string, []any, error) {
			return builder.NewSelectBuilder(r, "a").Columns("id").Join(builder.CrossJoin("", "b")).Build()
		}},
		{"in subquery nil query", func() (string, []any, error) {
			return builder.NewSelectBuilder(r, "t").Columns("id").Where(builder.InSubquery("id", nil)).Build()
		}},
		{"insert values and select", func() (string, []any, error) {
			source := builder.NewSelectBuilder(r, "s").Columns("id")
			return builder.NewInsertBuilder(r, "t").Columns("id").Values(1).FromSelect(source).Build()
		}},
		{"on duplicate key on non-mysql", func() (string, []any, error) {
			return builder.NewInsertBuilder(r, "t").Columns("id").Values(1).
				OnDuplicateKeyUpdate(builder.Assign("id", 2)).Build()
		}},
		{"conflict do update without assignments", func() (string, []any, error) {
			return builder.NewInsertBuilder(r, "t").Columns("id").Values(1).
				OnConflictDoUpdate([]string{"id"}).Build()
		}},
		{"cast without type", func() (string, []any, error) {
			return builder.NewSelectBuilder(r, "t").Projections(builder.Project(builder.Cast(builder.Column("a"), ""))).Build()
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

func TestBuilderRejectsRawSQLInjectionSurfaces(t *testing.T) {
	renderer := sqliteRenderer(t)
	tests := []struct {
		name  string
		build func() (string, []any, error)
	}{
		{"function", func() (string, []any, error) {
			return builder.NewSelectBuilder(renderer, "records").Projections(builder.Project(builder.Func("COUNT); DROP TABLE users;--", builder.Column("id")))).Build()
		}},
		{"cast", func() (string, []any, error) {
			return builder.NewSelectBuilder(renderer, "records").Projections(builder.Project(builder.Cast(builder.Column("id"), "TEXT); DROP TABLE users;--"))).Build()
		}},
		{"window frame", func() (string, []any, error) {
			return builder.NewSelectBuilder(renderer, "records").Projections(builder.Project(builder.Over(builder.RowNumber(), builder.Window().Frame("ROWS CURRENT ROW); DROP TABLE users;--")))).Build()
		}},
		{"row lock", func() (string, []any, error) {
			return builder.NewSelectBuilder(renderer, "records").Columns("id").ForUpdate("NOWAIT; DROP TABLE users").Build()
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if query, _, err := test.build(); err == nil || query != "" {
				t.Fatalf("unsafe SQL input was accepted: query=%q err=%v", query, err)
			}
		})
	}
}
