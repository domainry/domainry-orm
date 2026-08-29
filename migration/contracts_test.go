package migration

import "testing"

func TestMigrationDeclarationRetainsBaseline(t *testing.T) {
	value := Migration{
		Version: 1,
		Name:    "initial",
		Baseline: &Baseline{Tables: []Table{{
			Name:    "records",
			Columns: []Column{{Name: "id", Type: "TEXT", PrimaryKey: true}},
			Indexes: []Index{{Name: "idx_records", Columns: []string{"id"}}},
		}}},
	}
	if value.Baseline.Tables[0].Columns[0].Name != "id" || value.Baseline.Tables[0].Indexes[0].Name != "idx_records" {
		t.Fatalf("migration = %#v", value)
	}
}
