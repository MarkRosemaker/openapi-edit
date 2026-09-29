package edit_test

import (
	"slices"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

func mappingDoc(t *testing.T) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "components": {
    "schemas": {
      "Pet": {
        "oneOf": [
          {"$ref": "#/components/schemas/Cat"},
          {"$ref": "#/components/schemas/Dog"}
        ],
        "discriminator": {
          "propertyName": "kind",
          "mapping": {
            "meow": "Cat",
            "woof": "#/components/schemas/Dog",
            "purr": "Cat"
          }
        }
      },
      "Cat": {"type": "object"},
      "Dog": {"type": "object"},
      "Animal": {"type": "object"}
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

func mapping(doc *openapi.Document) []string {
	var got []string
	for k, v := range doc.Components.Schemas["Pet"].Discriminator.Mapping.ByIndex() {
		got = append(got, k+"="+v.Value)
	}

	return got
}

func TestRenameSchema_RewritesDiscriminatorMapping(t *testing.T) {
	doc := mappingDoc(t)

	if err := edit.RenameSchema(doc, "Cat", "Kitty"); err != nil {
		t.Fatal(err)
	}

	if err := edit.RenameSchema(doc, "Dog", "Hound"); err != nil {
		t.Fatal(err)
	}

	// each value keeps its form and its place, and the names that selected Cat and Dog implicitly still do
	want := []string{"meow=Kitty", "woof=#/components/schemas/Hound", "purr=Kitty", "Cat=Kitty", "Dog=Hound"}
	if got := mapping(doc); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRedirectSchema_RewritesDiscriminatorMapping(t *testing.T) {
	doc := mappingDoc(t)

	if err := edit.RedirectSchema(doc, "Cat", "Animal", ""); err != nil {
		t.Fatal(err)
	}

	// "Cat" also selected Cat by name, so it now maps to Animal explicitly
	want := []string{"meow=Animal", "woof=#/components/schemas/Dog", "purr=Animal", "Cat=Animal"}
	if got := mapping(doc); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}

func implicitDoc(t *testing.T) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "components": {
    "schemas": {
      "Pet": {
        "oneOf": [
          {"$ref": "#/components/schemas/Cat"},
          {"$ref": "#/components/schemas/Dog"}
        ],
        "discriminator": {"propertyName": "kind", "mapping": {"Dog": "Dog"}}
      },
      "Vehicle": {
        "allOf": [{"type": "object", "properties": {"kind": {"type": "string"}}}],
        "discriminator": {"propertyName": "kind"}
      },
      "Car": {"allOf": [{"$ref": "#/components/schemas/Vehicle"}, {"type": "object"}]},
      "Cat": {"type": "object"},
      "Dog": {"type": "object"},
      "Animal": {"type": "object"},
      "Other": {"type": "object"}
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

func mappingOf(doc *openapi.Document, name string) []string {
	var got []string
	for k, v := range doc.Components.Schemas[name].Discriminator.Mapping.ByIndex() {
		got = append(got, k+"="+v.Value)
	}

	return got
}

func TestRenameSchema_KeepsImplicitMapping(t *testing.T) {
	doc := implicitDoc(t)

	// a oneOf alternative, an allOf extension, one with an explicit entry already, and one no discriminator names
	for _, names := range [][2]string{{"Cat", "Kitty"}, {"Car", "Automobile"}, {"Dog", "Hound"}, {"Other", "Else"}} {
		if err := edit.RenameSchema(doc, names[0], names[1]); err != nil {
			t.Fatal(err)
		}
	}

	if got, want := mappingOf(doc, "Pet"), []string{"Dog=Hound", "Cat=Kitty"}; !slices.Equal(got, want) {
		t.Errorf("Pet: got %v, want %v", got, want)
	}

	if got, want := mappingOf(doc, "Vehicle"), []string{"Car=Automobile"}; !slices.Equal(got, want) {
		t.Errorf("Vehicle: got %v, want %v", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectSchema_KeepsImplicitMapping(t *testing.T) {
	doc := implicitDoc(t)

	if err := edit.RedirectSchema(doc, "Cat", "Animal", ""); err != nil {
		t.Fatal(err)
	}

	if got, want := mappingOf(doc, "Pet"), []string{"Dog=Dog", "Cat=Animal"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}
