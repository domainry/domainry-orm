# query

`query` assembles parameterized SQL statements for the three dialects that
`domainry-orm` supports: SQLite, PostgreSQL and MySQL. Every builder takes a
`dialect.Renderer` at construction time and returns `(sql string, args []any,
err error)` from `Build()`. Placeholders and identifier quoting follow the
renderer, so the same builder code produces `$N`/`"col"` for PostgreSQL, `?`/
`"col"` for SQLite and `?`/`` `col` `` for MySQL.

DDL and column declarations no longer belong to this package. Use `schema` for
infrastructure-owned tables and `recordschema` for Domainry Record tables.
The old DDL names remain temporarily as deprecated source-compatible facades.

```go
renderer, _ := dialect.ParseRenderer("postgres", "", "")
sql, args, err := query.NewSelectBuilder(renderer, "users").
    Columns("id", "email").
    Where(query.Equal("active", true)).
    Build()
// SELECT "id", "email" FROM "users" WHERE "active" = $1   args=[true]
```

All examples below are covered by `query/query_advanced_test.go` and
`query/query_sql_test.go`.

## Builders

- `NewSelectBuilder(renderer, table)` / `NewSelectFromSubquery(renderer, sub, alias)`
- `NewInsertBuilder(renderer, table)`
- `NewUpdateBuilder(renderer, table)`
- `NewDeleteBuilder(renderer, table)`

## SELECT

### Projections, filtering, grouping, ordering, paging

```go
query.NewSelectBuilder(r, "orders").
    Projections(
        query.Project(query.Column("user_id")),
        query.ProjectAs(query.Count(query.Column("id")), "n"),
    ).
    Where(query.GreaterThan("total", 0)).
    GroupBy(query.Column("user_id")).
    Having(query.GreaterThanExpression(query.Count(query.Column("id")), 1)).
    OrderBy(query.Descending("user_id")).
    Limit(20).Offset(40)
```

### DISTINCT and DISTINCT ON

`Distinct()` is portable. `DistinctOn(...)` renders `DISTINCT ON (...)` and is
PostgreSQL-specific.

```go
query.NewSelectBuilder(r, "events").
    DistinctOn(query.Column("user_id")).
    Projections(query.Project(query.Column("user_id")), query.Project(query.Column("created_at"))).
    OrderBy(query.Ascending("user_id"), query.Descending("created_at"))
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
query.NewSelectBuilder(r, "users").Alias("u").
    Projections(query.Project(query.QualifiedColumn("u", "id"))).
    Join(query.InnerJoinSubquery(sub, "o",
        query.EqualExpressions(query.QualifiedColumn("u", "id"), query.QualifiedColumn("o", "user_id"))))
```

### FROM a subquery (derived table)

```go
inner := query.NewSelectBuilder(r, "events").
    Projections(query.Project(query.Column("user_id")), query.ProjectAs(query.Count(query.Column("id")), "n")).
    GroupBy(query.Column("user_id"))

query.NewSelectFromSubquery(r, inner, "e").
    Projections(query.Project(query.QualifiedColumn("e", "user_id"))).
    Where(query.GreaterThanExpression(query.QualifiedColumn("e", "n"), 5))
// SELECT "e"."user_id" FROM (SELECT ...) AS "e" WHERE "e"."n" > ?
```

### CTEs — WITH / WITH RECURSIVE

```go
active := query.NewSelectBuilder(r, "users").Columns("id").Where(query.Equal("active", true))
query.NewSelectBuilder(r, "active_users").Columns("id").With("active_users", active)
// WITH "active_users" AS (SELECT ...) SELECT "id" FROM "active_users"
```

Pass optional column names as trailing args: `With("totals", src, "user_id", "n")`.
`WithRecursive(...)` emits `WITH RECURSIVE`.

### Set operations

`Union`, `UnionAll`, `Intersect`, `IntersectAll`, `Except`, `ExceptAll` compose
two selects. PostgreSQL placeholder numbering stays sequential across the
combined query.

```go
a.Union(b).OrderBy(query.Ascending("id")).Limit(10)
// SELECT ... UNION SELECT ... ORDER BY "id" ASC LIMIT ?
```

### Row locking

`ForUpdate`, `ForNoKeyUpdate`, `ForShare`, `ForKeyShare` accept optional
modifiers such as `"NOWAIT"` or `"SKIP LOCKED"`.

```go
query.NewSelectBuilder(r, "jobs").Columns("id").
    Where(query.Equal("state", "queued")).
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
query.NewSelectBuilder(r, "users").Columns("id").
    Where(query.InSubquery("id", bans))
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
query.ProjectAs(query.Filtered(query.CountAll(), query.Equal("status", "paid")), "paid_count")
// COUNT(*) FILTER (WHERE "status" = $1) AS "paid_count"
```

### Window functions

`Over(function, spec)` with `Window().PartitionBy(...).OrderBy(...).Frame(...)`.
Ranking helpers: `RowNumber`, `Rank`, `DenseRank`, `Lag`, `Lead`.

```go
query.Over(query.RowNumber(), query.Window().
    PartitionBy(query.Column("user_id")).
    OrderBy(query.Descending("created_at")))
// ROW_NUMBER() OVER (PARTITION BY "user_id" ORDER BY "created_at" DESC)

query.Over(query.Sum(query.Column("amount")), query.Window().
    OrderBy(query.Ascending("day")).
    Frame("ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW"))
```

## INSERT

```go
query.NewInsertBuilder(r, "users").
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
    query.Assign("email", "a@b.co"),
    query.AssignExpression("hits", query.Add(query.Column("hits"), query.Value(1))),
)
b.OnConflictDoUpdateWhere([]string{"id"}, query.GreaterThanExpression(query.Column("hits"), 0),
    query.AssignExpression("hits", query.Add(query.Column("hits"), query.Value(1))),
)
```

MySQL:

```go
b.OnDuplicateKeyUpdate(query.Assign("email", "a@b.co"))
// ON DUPLICATE KEY UPDATE `email` = ?
```

`OnDuplicateKeyUpdate` returns an error unless the renderer is MySQL.

## UPDATE / DELETE

```go
query.NewUpdateBuilder(r, "users").
    Set("name", "Ann").
    SetExpression("hits", query.Add(query.Column("hits"), query.Value(1))).
    Where(query.Equal("id", "u1")).
    Returning("*")

query.NewDeleteBuilder(r, "sessions").
    Where(query.Equal("id", "s1")).
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
