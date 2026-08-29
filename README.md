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
- `migration`: source-owned migration and physical baseline declarations.
