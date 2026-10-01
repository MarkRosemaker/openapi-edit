package edit_test

import (
	"slices"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

func alternatives(l openapi.SchemaList) []string {
	var names []string
	for _, a := range l {
		names = append(names, a.Ref.Identifier[len("#/components/schemas/"):])
	}

	return names
}

func TestRedirectSchema_DropsDuplicateAlternatives(t *testing.T) {
	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "components": {
    "schemas": {
      "Pet": {"oneOf": [{"$ref": "#/components/schemas/Cat"}, {"$ref": "#/components/schemas/Dog"}]},
      "Any": {"anyOf": [
        {"$ref": "#/components/schemas/Dog"},
        {"$ref": "#/components/schemas/Bird"},
        {"$ref": "#/components/schemas/Cat"},
        {"$ref": "#/components/schemas/Dog", "maxProperties": 1}
      ]},
      "Untouched": {"anyOf": [{"$ref": "#/components/schemas/Bird"}, {"$ref": "#/components/schemas/Bird"}]},
      "Cat": {"type": "object"},
      "Dog": {"type": "object"},
      "Bird": {"type": "object"}
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	if err := edit.RedirectSchema(doc, "Dog", "Cat", ""); err != nil {
		t.Fatal(err)
	}

	s := doc.Components.Schemas

	// a oneOf of the same schema twice could never hold, so one is enough
	if got, want := alternatives(s["Pet"].OneOf), []string{"Cat"}; !slices.Equal(got, want) {
		t.Errorf("Pet: got %v, want %v", got, want)
	}

	// the first stays where it was; one that says more than the schema it refers to is no duplicate
	if got, want := alternatives(s["Any"].AnyOf), []string{"Cat", "Bird", "Cat"}; !slices.Equal(got, want) {
		t.Errorf("Any: got %v, want %v", got, want)
	}

	if s["Any"].AnyOf[2].MaxProperties == nil {
		t.Errorf("Any: the alternative with maxProperties was dropped instead of the plain one")
	}

	// a redirect only tidies what it changed
	if got, want := alternatives(s["Untouched"].AnyOf), []string{"Bird", "Bird"}; !slices.Equal(got, want) {
		t.Errorf("Untouched: got %v, want %v", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}
