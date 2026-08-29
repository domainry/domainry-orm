# builder

`builder` assembles parameterized SQL statements for the three dialects that
`domainry-orm` supports: SQLite, PostgreSQL and MySQL. Every builder takes a
`dialect.Renderer` at construction time and returns `(sql string, args []any,
err error)` from `Build()`. Placeholders and identifier quoting follow the
renderer, so the same builder code produces `$N`/`"col"` for PostgreSQL, `?`/
`"col"` for SQLite and `?`/`` `col` `` for MySQL.

## Schema construction

`NewCreateTableBuilder` always materializes the Domainry system columns below,
even when the schema owner supplies no business columns:

- `workspace_id`, `id`
- `created_at`, `updated_at`
- `deleted`, `ext_info`
- `create_user_id`, `update_user_id`

Schema owners may still mention a system column explicitly when adopting a
pre-existing physical contract; the builder never emits it twice. New schemas
should omit these columns and declare only their business-owned fields.
Infrastructure-owned tables with a different physical contract must opt out
explicitly with `WithoutSystemColumns()` and declare every column themselves.

Portable column constructors cover bounded and unbounded text, small/integer/
big integer, exact decimal, real/double, boolean, date/time/timestamp, JSON,
UUID, and binary values. Rendering preserves the stronger native type where a
dialect has one (`JSONB`, `UUID`, and `BYTEA` on PostgreSQL; `JSON` and
`LONGBLOB` on MySQL) and uses the corresponding SQLite storage class.

```go
renderer, _ := dialect.ParseRenderer("postgres", "", "")
sql, args, err := builder.NewSelectBuilder(renderer, "users").
    Columns("id", "email").
    Where(builder.Equal("active", true)).
    Build()
// SELECT "id", "email" FROM "users" WHERE "active" = $1   args=[true]
```

All examples below are covered by `builder/builder_advanced_test.go` and
`builder/builder_sql_test.go`.

## Builders

- `NewSelectBuilder(renderer, table)` / `NewSelectFromSubquery(renderer, sub, alias)`
- `NewInsertBuilder(renderer, table)`
- `NewUpdateBuilder(renderer, table)`
- `NewDeleteBuilder(renderer, table)`

## SELECT

### Projections, filtering, grouping, ordering, paging

```go
builder.NewSelectBuilder(r, "orders").
    Projections(
        builder.Project(builder.Column("user_id")),
        builder.ProjectAs(builder.Count(builder.Column("id")), "n"),
    ).
    Where(builder.GreaterThan("total", 0)).
    GroupBy(builder.Column("user_id")).
    Having(builder.GreaterThanExpression(builder.Count(builder.Column("id")), 1)).
    OrderBy(builder.Descending("user_id")).
    Limit(20).Offset(40)
```

### DISTINCT and DISTINCT ON

`Distinct()` is portable. `DistinctOn(...)` renders `DISTINCT ON (...)` and is
PostgreSQL-specific.

```go
builder.NewSelectBuilder(r, "events").
    DistinctOn(builder.Column("user_id")).
    Projections(builder.Project(builder.Column("user_id")), builder.Project(builder.Column("created_at"))).
    OrderBy(builder.Ascending("user_id"), builder.Descending("created_at"))
// SELECT DISTINCT ON ("user_id") "user_id", "created_at" FROM "events" ORDER BY ...
```

### Grouping extensions

`GroupByRollup`, `GroupByCube` and `GroupByGroupingSets` emit
`GROUP BY ROLLUP (...)`, `GROUP BY CUBE (...)` and `GROUP BY GROUPING SETS (...)`
(PostgreSQL / MySQL grouping extensions).

### Joins

`InnerJoin`, `LeftJoin`, `RightJoin`, `FullJoin` take `(table, alias, predicate)`.
`CrossJoin(table, alias)` has no `ON` clause. Each has a `*JoinSubquery` variant
that joins a derived table, and `.Lateral()` prefixes `LATERAL`.

```go
builder.NewSelectBuilder(r, "users").Alias("u").
    Projections(builder.Project(builder.QualifiedColumn("u", "id"))).
    Join(builder.InnerJoinSubquery(sub, "o",
        builder.EqualExpressions(builder.QualifiedColumn("u", "id"), builder.QualifiedColumn("o", "user_id"))))
```

### FROM a subquery (derived table)

```go
inner := builder.NewSelectBuilder(r, "events").
    Projections(builder.Project(builder.Column("user_id")), builder.ProjectAs(builder.Count(builder.Column("id")), "n")).
    GroupBy(builder.Column("user_id"))

builder.NewSelectFromSubquery(r, inner, "e").
    Projections(builder.Project(builder.QualifiedColumn("e", "user_id"))).
    Where(builder.GreaterThanExpression(builder.QualifiedColumn("e", "n"), 5))
// SELECT "e"."user_id" FROM (SELECT ...) AS "e" WHERE "e"."n" > ?
```

### CTEs — WITH / WITH RECURSIVE

```go
active := builder.NewSelectBuilder(r, "users").Columns("id").Where(builder.Equal("active", true))
builder.NewSelectBuilder(r, "active_users").Columns("id").With("active_users", active)
// WITH "active_users" AS (SELECT ...) SELECT "id" FROM "active_users"
```

Pass optional column names as trailing args: `With("totals", src, "user_id", "n")`.
`WithRecursive(...)` emits `WITH RECURSIVE`.

### Set operations

`Union`, `UnionAll`, `Intersect`, `IntersectAll`, `Except`, `ExceptAll` compose
two selects. PostgreSQL placeholder numbering stays sequential across the
combined query.

```go
a.Union(b).OrderBy(builder.Ascending("id")).Limit(10)
// SELECT ... UNION SELECT ... ORDER BY "id" ASC LIMIT ?
```

### Row locking

`ForUpdate`, `ForNoKeyUpdate`, `ForShare`, `ForKeyShare` accept optional
modifiers such as `"NOWAIT"` or `"SKIP LOCKED"`.

```go
builder.NewSelectBuilder(r, "jobs").Columns("id").
    Where(builder.Equal("state", "queued")).
    ForUpdate("SKIP LOCKED")
// SELECT "id" FROM "jobs" WHERE "state" = ? FOR UPDATE SKIP LOCKED
```

## Predicates

Column helpers (`Equal`, `NotEqual`, `LessThan`, `GreaterThan`, ...), expression
helpers (`EqualExpressions`, `GreaterThanExpression`, ...), `Between`, `In` /
`NotIn`, `Like` / `NotLike`, `IsNull` / `IsNotNull`, and `And` / `Or`.

### Subquery predicates

- `InSubquery` / `NotInSubquery` (and `*Expression` variants)
- scalar comparisons: `EqualSubquery`, `NotEqualSubquery`, `LessThanSubquery`,
  `LessThanOrEqualSubquery`, `GreaterThanSubquery`, `GreaterThanOrEqualSubquery`
- `ExistsSubquery` / `NotExistsSubquery`

```go
builder.NewSelectBuilder(r, "users").Columns("id").
    Where(builder.InSubquery("id", bans))
// SELECT "id" FROM "users" WHERE "id" IN (SELECT ...)
```

### Case-insensitive LIKE (dialect-aware)

`ILike` / `NotILike` (and `*Expression` variants) render native `ILIKE` on
PostgreSQL and fall back to `LOWER(x) LIKE LOWER(?)` on MySQL and SQLite.

| Dialect | `ILike("email", "%@X.com")` |
| --- | --- |
| PostgreSQL | `"email" ILIKE $1` |
| MySQL | `` LOWER(`email`) LIKE LOWER(?) `` |
| SQLite | `LOWER("email") LIKE LOWER(?)` |

## Ordering NULLs (dialect-aware)

`Ascending(...).NullsLast()` / `.NullsFirst()` emit native `NULLS FIRST/LAST` on
PostgreSQL and SQLite. MySQL has no such syntax and is emulated with a leading
`ISNULL(col)` sort key.

| Dialect | `Ascending("score").NullsLast()` |
| --- | --- |
| PostgreSQL / SQLite | `"score" ASC NULLS LAST` |
| MySQL | `` ISNULL(`score`) ASC, `score` ASC `` |

## Expressions

Columns and values: `Column`, `QualifiedColumn`, `TableColumn`, `Value`, `Star`.

Arithmetic: `Add`, `Subtract`, `Multiply`, `Divide`.

Aggregates: `Count`, `CountAll`, `Sum`, `Avg`, `Max`, `Min`, `Coalesce`.

Scalar functions: `Lower`, `Upper`, `Abs`, `Length`, `Cast(expr, "date")`, and
the generic `Func(name, args...)`.

`CaseWhen(pred, then).When(pred, then).Else(result)` builds `CASE WHEN ... END`.

### Filtered aggregates

`Filtered(aggregate, predicate)` renders `agg FILTER (WHERE predicate)`.

```go
builder.ProjectAs(builder.Filtered(builder.CountAll(), builder.Equal("status", "paid")), "paid_count")
// COUNT(*) FILTER (WHERE "status" = $1) AS "paid_count"
```

### Window functions

`Over(function, spec)` with `Window().PartitionBy(...).OrderBy(...).Frame(...)`.
Ranking helpers: `RowNumber`, `Rank`, `DenseRank`, `Lag`, `Lead`.

```go
builder.Over(builder.RowNumber(), builder.Window().
    PartitionBy(builder.Column("user_id")).
    OrderBy(builder.Descending("created_at")))
// ROW_NUMBER() OVER (PARTITION BY "user_id" ORDER BY "created_at" DESC)

builder.Over(builder.Sum(builder.Column("amount")), builder.Window().
    OrderBy(builder.Ascending("day")).
    Frame("ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW"))
```

## INSERT

```go
builder.NewInsertBuilder(r, "users").
    Columns("id", "email").Values("u1", "a@b.co").
    Returning("id", "created_at")
```

- `Returning(cols...)` — supported by PostgreSQL and SQLite.
- `FromSelect(src)` — `INSERT ... SELECT` (mutually exclusive with `Values`).

### Upsert

PostgreSQL / SQLite:

```go
b.OnConflictDoNothing("id")
b.OnConflictDoUpdate([]string{"id"},
    builder.Assign("email", "a@b.co"),
    builder.AssignExpression("hits", builder.Add(builder.Column("hits"), builder.Value(1))),
)
b.OnConflictDoUpdateWhere([]string{"id"}, builder.GreaterThanExpression(builder.Column("hits"), 0),
    builder.AssignExpression("hits", builder.Add(builder.Column("hits"), builder.Value(1))),
)
```

MySQL:

```go
b.OnDuplicateKeyUpdate(builder.Assign("email", "a@b.co"))
// ON DUPLICATE KEY UPDATE `email` = ?
```

`OnDuplicateKeyUpdate` returns an error unless the renderer is MySQL.

## UPDATE / DELETE

```go
builder.NewUpdateBuilder(r, "users").
    Set("name", "Ann").
    SetExpression("hits", builder.Add(builder.Column("hits"), builder.Value(1))).
    Where(builder.Equal("id", "u1")).
    Returning("*")

builder.NewDeleteBuilder(r, "sessions").
    Where(builder.Equal("id", "s1")).
    Returning("id")
```

`Assign` / `AssignExpression` produce the shared `Assignment` values used by
`UpdateBuilder`, `OnConflictDoUpdate` and `OnDuplicateKeyUpdate`.

## Dialect behavior summary

| Feature | SQLite | PostgreSQL | MySQL |
| --- | --- | --- | --- |
| Placeholder | `?` | `$N` | `?` |
| Identifier quote | `"` | `"` | `` ` `` |
| `ILIKE` | emulated | native | emulated |
| `NULLS FIRST/LAST` | native | native | emulated (`ISNULL`) |
| `DISTINCT ON` | — | native | — |
| `RETURNING` | native | native | — |
| `ON CONFLICT` | native | native | — |
| `ON DUPLICATE KEY UPDATE` | — | — | native |

Dialect-specific clauses are emitted verbatim; issuing them against an
unsupported engine is a caller error rather than a builder error, except for
`OnDuplicateKeyUpdate`, which validates the dialect.
