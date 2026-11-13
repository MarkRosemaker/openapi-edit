package edit_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

// parse reads a document of the paths and schemas given, as JSON.
func parse(t *testing.T, paths, schemas string) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromData([]byte(`{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":` + paths +
		`,"components":{"schemas":` + schemas + `}}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

func TestSchemaRef(t *testing.T) {
	doc := parse(t, `{}`, `{"Pet":{"type":"object"}}`)

	ref := edit.SchemaRef(doc, "Pet")
	if ref.Ref.Identifier != "#/components/schemas/Pet" || ref.Ref.Value != doc.Components.Schemas["Pet"] {
		t.Errorf("got %+v", ref.Ref)
	}
}

func TestRenameSchemasFunc(t *testing.T) {
	doc := parse(t, `{}`, `{
		"PetRequest":{"type":"object","properties":{"owner":{"$ref":"#/components/schemas/OwnerRequest"}}},
		"OwnerRequest":{"type":"object"},
		"Kind":{"type":"string"}
	}`)

	if err := edit.RenameSchemasFunc(doc, func(n string) string { return strings.TrimSuffix(n, "Request") }); err != nil {
		t.Fatal(err)
	}

	if got := slices.Collect(func(yield func(string) bool) {
		for n := range doc.Components.Schemas.ByIndex() {
			if !yield(n) {
				return
			}
		}
	}); !slices.Equal(got, []string{"Pet", "Owner", "Kind"}) {
		t.Errorf("got %q", got)
	}

	if ref := doc.Components.Schemas["Pet"].Properties["owner"].Ref; ref.Identifier != "#/components/schemas/Owner" {
		t.Errorf("reference left at %s", ref.Identifier)
	}

	doc = parse(t, `{}`, `{"PetRequest":{"type":"object"},"Pet":{"type":"object"}}`)

	var exists *edit.ErrSchemaExists
	if err := edit.RenameSchemasFunc(doc, func(n string) string { return strings.TrimSuffix(n, "Request") }); !errors.As(err, &exists) {
		t.Errorf("onto a name taken: got %v", err)
	}
}
