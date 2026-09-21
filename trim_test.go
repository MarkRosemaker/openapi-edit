package edit_test

import (
	"encoding/json/jsontext"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

func TestTrimExample(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		v        string
		maxItems int
		want     string
	}{
		{
			name:     "array shorter than the budget is untouched",
			v:        `[1,2]`,
			maxItems: 3,
			want:     `[1,2]`,
		},
		{
			name:     "one element per distinct object shape, in order, before padding",
			v:        `[{"a":1},{"a":2,"b":3},{"a":4},{"a":5,"b":6},{"a":7}]`,
			maxItems: 2,
			// element 0 ({"a":1}) covers shape {a}; element 1 ({"a":2,"b":3})
			// covers shape {a,b}. Budget of 2 is spent covering both shapes,
			// so nothing is left to pad with.
			want: `[{"a":1},{"a":2,"b":3}]`,
		},
		{
			name: "fewer distinct shapes than the budget pads with what comes next",
			v:    `[1,2,3,4]`,
			// every element has the same shape ("number"): the first pass
			// keeps only element 0, so the second pass pads with element 1.
			maxItems: 2,
			want:     `[1,2]`,
		},
		{
			name:     "a value that is sometimes a number and sometimes a string survives",
			v:        `[1,2,3,"Infinity",4]`,
			maxItems: 2,
			want:     `[1,"Infinity"]`,
		},
		{
			name:     "object key order is preserved",
			v:        `{"z":1,"a":2,"m":3}`,
			maxItems: 3,
			want:     `{"z":1,"a":2,"m":3}`,
		},
		{
			name:     "recurses into a kept element's own nested array",
			v:        `[{"id":1,"tags":[10,20,30,40]},{"id":2,"tags":[10,20,30,40]}]`,
			maxItems: 1,
			want:     `[{"id":1,"tags":[10]}]`,
		},
		{
			name:     "a bare scalar has nothing to trim",
			v:        `"foo"`,
			maxItems: 3,
			want:     `"foo"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := edit.TrimExample(jsontext.Value(tc.v), tc.maxItems)
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestTrimExample_DefaultsMaxItems(t *testing.T) {
	t.Parallel()

	got, err := edit.TrimExample(jsontext.Value(`[1,2,3,4,5]`), 0)
	if err != nil {
		t.Fatal(err)
	}

	want := `[1,2,3]`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestTrimExample_InvalidJSON(t *testing.T) {
	t.Parallel()

	if _, err := edit.TrimExample(jsontext.Value(`{not json`), 3); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTrimSchemaExamples(t *testing.T) {
	t.Parallel()

	item := &openapi.Schema{
		Type:    openapi.TypeInteger,
		Example: jsontext.Value(`[1,2,3,4,5]`),
	}

	array := &openapi.Schema{
		Type:  openapi.TypeArray,
		Items: &openapi.SchemaRef{Value: item},
	}

	d := doc(array)

	if err := edit.TrimSchemaExamples(d, 2); err != nil {
		t.Fatal(err)
	}

	want := `[1,2]`
	if string(item.Example) != want {
		t.Errorf("got %s, want %s", item.Example, want)
	}
}

// TestTrimSchemaExamples_UnreferencedComponentSchema covers a component
// schema nothing else in the document references: components.schemas holds
// *openapi.Schema directly, with no enclosing SchemaRef of its own, so it
// would never reach walkSchemaRefs' fn on its own.
func TestTrimSchemaExamples_UnreferencedComponentSchema(t *testing.T) {
	t.Parallel()

	target := &openapi.Schema{
		Type:    openapi.TypeInteger,
		Example: jsontext.Value(`[1,2,3,4,5]`),
	}

	d := doc(target)

	if err := edit.TrimSchemaExamples(d, 2); err != nil {
		t.Fatal(err)
	}

	want := `[1,2]`
	if string(target.Example) != want {
		t.Errorf("got %s, want %s", target.Example, want)
	}
}

func TestTrimSchemaExamples_LeavesSchemasWithoutExampleAlone(t *testing.T) {
	t.Parallel()

	target := &openapi.Schema{Type: openapi.TypeObject}
	d := doc(target)

	if err := edit.TrimSchemaExamples(d, 3); err != nil {
		t.Fatal(err)
	}

	if target.Example != nil {
		t.Errorf("got %s, want nil", target.Example)
	}
}
