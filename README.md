# Domainry ORM

`domainry-orm` provides the small, domain-neutral SQL infrastructure shared by
Domainry services and embedded modules.

It owns SQL execution contracts, safe identifier handling, schema-qualified
table names, placeholders, and statement fragments. Product repositories,
business schemas, migrations, workspace policy, and transaction workflows stay
with their owning modules.

## Packages

- `sqlhost`: narrow `database/sql` execution and transaction contracts.
- `dialect`: portable SQL identifier, table, placeholder, and insert behavior.
- `builder`: parameterized SELECT/INSERT/UPDATE/DELETE construction across
  SQLite, PostgreSQL and MySQL. See [`builder/README.md`](builder/README.md).
- `schema`: portable, domain-neutral table, column, index, and ALTER declarations.
- `recordschema`: Domainry Record table conventions, including workspace and
  audit system columns.
- `batch`: database-parameter budget and statement batch calculation.
- `migration`: source-owned migration and physical baseline declarations.
- `postgres`: PostgreSQL capability probes, retry, and safe failure classification.
