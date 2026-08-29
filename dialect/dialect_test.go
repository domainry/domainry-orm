package dialect

import "testing"

func TestDialectRendering(t *testing.T) {
	tests := []struct {
		name        Name
		table       string
		placeholder string
		insert      string
	}{
		{name: SQLite, table: `"records"`, placeholder: "?", insert: `INSERT INTO "records" ("id", "name") VALUES (?, ?)`},
		{name: MySQL, table: "`records`", placeholder: "?", insert: "INSERT INTO `records` (`id`, `name`) VALUES (?, ?)"},
		{name: Postgres, table: `"runtime"."records"`, placeholder: "$2", insert: `INSERT INTO "runtime"."records" ("id", "name") VALUES ($1, $2)`},
	}
	for _, test := range tests {
		t.Run(string(test.name), func(t *testing.T) {
			dialect, err := New(test.name)
			if err != nil {
				t.Fatal(err)
			}
			if got := dialect.Table("runtime", "records"); got != test.table {
				t.Fatalf("table = %q, want %q", got, test.table)
			}
			if got := dialect.Placeholder(2); got != test.placeholder {
				t.Fatalf("placeholder = %q, want %q", got, test.placeholder)
			}
			if got := dialect.Insert("runtime", "records", "id", "name"); got != test.insert {
				t.Fatalf("insert = %q, want %q", got, test.insert)
			}
		})
	}
}

func TestInvalidInputsAreRejected(t *testing.T) {
	if _, err := New(Name("oracle")); err == nil {
		t.Fatal("unsupported dialect accepted")
	}
	for _, value := range []string{"", "1record", "record-name", `record"name`, "record.name"} {
		if ValidIdentifier(value) {
			t.Fatalf("invalid identifier %q accepted", value)
		}
	}
	assertPanics(t, func() {
		dialect, _ := New(Postgres)
		_ = dialect.Identifier("records;drop_table")
	})
	assertPanics(t, func() {
		dialect, _ := New(Postgres)
		_ = dialect.Placeholder(0)
	})
}

func TestParseAndSchemaBoundRenderer(t *testing.T) {
	postgres, err := Parse("pgx")
	if err != nil {
		t.Fatal(err)
	}
	renderer := postgres.WithSchema("tenant")
	if got := renderer.Table("records"); got != `"tenant"."records"` {
		t.Fatalf("table = %q", got)
	}
	if got := renderer.Insert("records", []string{"id", "name"}); got != `INSERT INTO "tenant"."records" ("id", "name") VALUES ($1, $2)` {
		t.Fatalf("insert = %q", got)
	}
	if sqlite, err := Parse("sqlite3"); err != nil || sqlite.Name() != SQLite {
		t.Fatalf("sqlite alias = %q, %v", sqlite.Name(), err)
	}
}

func assertPanics(t *testing.T, run func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	run()
}
