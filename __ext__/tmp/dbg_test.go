package builder_test

import (
	"testing"

	"github.com/domainry/domainry-orm/dialect"
	"github.com/domainry/domainry-orm/query"
)

func TestDebugCountAll(t *testing.T) {
	r, _ := dialect.ParseRenderer("sqlite", "", "")
	s, a, err := query.NewSelectBuilder(r, "items").Projections(
		query.Project(query.CountAll()),
	).Build()
	t.Logf("sql=%q args=%v err=%v", s, a, err)
}
