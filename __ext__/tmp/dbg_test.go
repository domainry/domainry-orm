package builder_test

import (
	"testing"

	"github.com/domainry/domainry-orm/builder"
	"github.com/domainry/domainry-orm/dialect"
)

func TestDebugCountAll(t *testing.T) {
	r, _ := dialect.ParseRenderer("sqlite", "", "")
	s, a, err := builder.NewSelectBuilder(r, "items").Projections(
		builder.Project(builder.CountAll()),
	).Build()
	t.Logf("sql=%q args=%v err=%v", s, a, err)
}
