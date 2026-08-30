# schema

`schema` declares portable, domain-neutral DDL. It renders table, column,
index, and ALTER operations for SQLite, PostgreSQL, and MySQL without adding
product or Record conventions.

```go
table := schema.NewTable(renderer, "migration_lock").
    Columns(
        schema.Column("name", schema.TextKey(255)).NotNull(),
        schema.Column("acquired_at", schema.Timestamp()).NotNull(),
    ).
    PrimaryKey("name")
```

Use `recordschema.NewTable` when a table is a Domainry Record and therefore
requires the canonical workspace, identity, timestamp, deletion, extension,
and actor columns.
