# builder compatibility facade

New code should use the focused ORM packages:

- `query` for parameterized DML and predicates;
- `schema` for generic DDL;
- `recordschema` for Domainry Record table conventions;
- `batch` for parameter-budget batching.

`builder` retains deprecated aliases so released modules can migrate imports
without a flag day.
