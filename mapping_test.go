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

	// each value keeps its form and its place
	want := []string{"meow=Kitty", "woof=#/components/schemas/Hound", "purr=Kitty"}
	if got := mapping(doc); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRedirectSchema_RewritesDiscriminatorMapping(t *testing.T) {
	doc := mappingDoc(t)

	if err := edit.RedirectSchema(doc, "Cat", "Animal", ""); err != nil {
		t.Fatal(err)
	}

	want := []string{"meow=Animal", "woof=#/components/schemas/Dog", "purr=Animal"}
	if got := mapping(doc); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}
