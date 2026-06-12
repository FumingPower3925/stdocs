package spec

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs/internal/schema"
)

// randomSchema builds an arbitrary-but-valid schema tree from a
// seeded source, exercising every facet the emitters render.
func randomSchema(r *rand.Rand, depth int) *schema.Schema {
	kinds := []string{"string", "integer", "number", "boolean", "array", "object"}
	s := &schema.Schema{Type: kinds[r.Intn(len(kinds))], Nullable: r.Intn(2) == 0}
	switch s.Type {
	case "string":
		if r.Intn(2) == 0 {
			n := uint64(r.Intn(5))
			s.MinLength = &n
		}
		if r.Intn(2) == 0 {
			s.Pattern = "^a"
		}
		if r.Intn(2) == 0 {
			s.Enum = []any{"a", "b"}
		}
		if r.Intn(2) == 0 {
			s.Example = "x"
		}
	case "integer", "number":
		switch r.Intn(3) {
		case 0:
			s.Minimum = "0"
		case 1:
			s.ExclusiveMinimum = "0"
		}
		if r.Intn(2) == 0 {
			s.Maximum = "100"
		}
		if r.Intn(2) == 0 {
			s.Default = int64(3)
		}
	case "array":
		if depth > 0 {
			s.Items = randomSchema(r, depth-1)
		} else {
			s.Items = &schema.Schema{Type: "string"}
		}
		if r.Intn(2) == 0 {
			s.UniqueItems = true
		}
	case "object":
		s.Properties = map[string]*schema.Schema{}
		for i := range r.Intn(3) + 1 {
			name := string(rune('a' + i))
			if depth > 0 {
				s.Properties[name] = randomSchema(r, depth-1)
			} else {
				s.Properties[name] = &schema.Schema{Type: "boolean"}
			}
			if r.Intn(2) == 0 {
				s.Required = append(s.Required, name)
			}
		}
	}
	if r.Intn(4) == 0 {
		s.Description = "d"
	}
	return s
}

// TestEmitterVersionInvariants drives both schema builders over a
// large generated corpus and asserts the per-version dialect rules
// hold everywhere in the output tree.
func TestEmitterVersionInvariants(t *testing.T) {
	r := rand.New(rand.NewSource(42)) // deterministic corpus
	for i := range 500 {
		s := randomSchema(r, 3)

		b30, err := json.Marshal(buildSchema30(s))
		if err != nil {
			t.Fatalf("case %d: 3.0 output does not marshal: %v", i, err)
		}
		b31, err := json.Marshal(buildSchema31(s))
		if err != nil {
			t.Fatalf("case %d: 3.1 output does not marshal: %v", i, err)
		}
		out30, out31 := string(b30), string(b31)

		// 3.0 dialect: no type arrays, no anyOf-null nullability, no
		// examples arrays, exclusive bounds as booleans.
		if strings.Contains(out30, `"type":[`) {
			t.Fatalf("case %d: 3.0 emitted a type array: %s", i, out30)
		}
		if strings.Contains(out30, `"examples"`) {
			t.Fatalf("case %d: 3.0 emitted an examples array: %s", i, out30)
		}
		if strings.Contains(out30, `"exclusiveMinimum":0`) || strings.Contains(out30, `"exclusiveMinimum":"0"`) {
			t.Fatalf("case %d: 3.0 exclusive bound must be boolean: %s", i, out30)
		}

		// 3.1 dialect: no nullable keyword, no type arrays (anyOf
		// form instead), no singular example, numeric exclusives.
		if strings.Contains(out31, `"nullable"`) {
			t.Fatalf("case %d: 3.1 emitted nullable: %s", i, out31)
		}
		if strings.Contains(out31, `"type":[`) {
			t.Fatalf("case %d: 3.1 emitted a type array: %s", i, out31)
		}
		if strings.Contains(out31, `"example":`) {
			t.Fatalf("case %d: 3.1 emitted singular example: %s", i, out31)
		}
		if strings.Contains(out31, `"exclusiveMinimum":true`) {
			t.Fatalf("case %d: 3.1 exclusive bound must be numeric: %s", i, out31)
		}

		// 3.2 shares the 3.1 schema dialect by construction.
		b32, _ := json.Marshal(buildSchema32(s))
		if string(b32) != out31 {
			t.Fatalf("case %d: 3.2 schema output diverged from 3.1", i)
		}

		// Determinism.
		again30, _ := json.Marshal(buildSchema30(s))
		if string(again30) != out30 {
			t.Fatalf("case %d: non-deterministic 3.0 emission", i)
		}
	}
}
