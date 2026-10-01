package edit_test

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

// batchDoc has references, discriminator mappings by name and by reference, implicit mappings through oneOf and
// allOf, and unions that a redirect leaves with duplicate alternatives.
func batchDoc(t *testing.T) *openapi.Document {
	t.Helper()

	doc, err := openapi.LoadFromDataJSON([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "t", "version": "1"},
  "paths": {
    "/pets": {"get": {"responses": {"200": {"description": "ok", "content": {"application/json": {
      "schema": {"type": "array", "items": {"$ref": "#/components/schemas/Dog"}}
    }}}}}}
  },
  "components": {
    "schemas": {
      "Pet": {
        "oneOf": [
          {"$ref": "#/components/schemas/Cat"},
          {"$ref": "#/components/schemas/Dog"},
          {"$ref": "#/components/schemas/Bird"}
        ],
        "discriminator": {"propertyName": "kind", "mapping": {"meow": "Cat", "woof": "#/components/schemas/Dog"}}
      },
      "Vehicle": {
        "type": "object",
        "properties": {"kind": {"type": "string"}},
        "discriminator": {"propertyName": "kind"}
      },
      "Car": {"allOf": [{"$ref": "#/components/schemas/Vehicle"}, {"type": "object"}]},
      "Owner": {"type": "object", "properties": {
        "pet": {"$ref": "#/components/schemas/Cat"},
        "car": {"$ref": "#/components/schemas/Car"}
      }},
      "Cat": {"type": "object"},
      "Dog": {"type": "object"},
      "Bird": {"type": "object"},
      "Animal": {"type": "object"}
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}

func toJSON(t *testing.T, doc *openapi.Document) string {
	t.Helper()

	b, err := doc.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func TestRedirectSchemas_SameAsOneByOne(t *testing.T) {
	to := map[string]string{"Cat": "Animal", "Dog": "Animal", "Bird": "Bird"}

	want := batchDoc(t)
	for _, oldName := range slices.Sorted(maps.Keys(to)) {
		if err := edit.RedirectSchema(want, oldName, to[oldName], ""); err != nil {
			t.Fatal(err)
		}
	}

	got := batchDoc(t)
	if err := edit.RedirectSchemas(got, to); err != nil {
		t.Fatal(err)
	}

	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}

	if g, w := toJSON(t, got), toJSON(t, want); g != w {
		t.Errorf("got\n%s\nwant\n%s", g, w)
	}

	if got, want := alternatives(got.Components.Schemas["Pet"].OneOf), []string{"Animal", "Bird"}; !slices.Equal(got, want) {
		t.Errorf("Pet: got %v, want %v", got, want)
	}
}

func TestRenameSchemas_SameAsOneByOne(t *testing.T) {
	to := map[string]string{"Cat": "Kitty", "Dog": "Hound", "Car": "Automobile", "Animal": "Animal"}

	want := batchDoc(t)
	for _, oldName := range slices.Sorted(maps.Keys(to)) {
		if err := edit.RenameSchema(want, oldName, to[oldName]); err != nil {
			t.Fatal(err)
		}
	}

	got := batchDoc(t)
	if err := edit.RenameSchemas(got, to); err != nil {
		t.Fatal(err)
	}

	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}

	if g, w := toJSON(t, got), toJSON(t, want); g != w {
		t.Errorf("got\n%s\nwant\n%s", g, w)
	}
}

func TestRenameSchemas_Swap(t *testing.T) {
	doc := batchDoc(t)
	cat, dog := doc.Components.Schemas["Cat"], doc.Components.Schemas["Dog"]

	if err := edit.RenameSchemas(doc, map[string]string{"Cat": "Dog", "Dog": "Cat"}); err != nil {
		t.Fatal(err)
	}

	s := doc.Components.Schemas
	if s["Dog"] != cat || s["Cat"] != dog {
		t.Error("the schemas did not swap names")
	}

	if got := s["Owner"].Properties["pet"]; got.Ref.Identifier != "#/components/schemas/Dog" || got.Ref.Value != cat {
		t.Errorf("Owner.pet = %q, want the schema that was Cat, now Dog", got.Ref.Identifier)
	}

	want := []string{"meow=Dog", "woof=#/components/schemas/Cat", "Cat=Dog", "Dog=Cat"}
	if got := mappingOf(doc, "Pet"); !slices.Equal(got, want) {
		t.Errorf("Pet: got %v, want %v", got, want)
	}

	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBatch_Errors(t *testing.T) {
	notFound := func(err error) bool {
		var e *edit.ErrSchemaNotFound
		return errors.As(err, &e)
	}
	exists := func(err error) bool {
		var e *edit.ErrSchemaExists
		return errors.As(err, &e)
	}

	for _, tc := range []struct {
		name string
		fn   func(*openapi.Document) error
		want func(error) bool
	}{{
		name: "redirecting onto a schema that is itself redirected",
		fn: func(d *openapi.Document) error {
			return edit.RedirectSchemas(d, map[string]string{"Cat": "Dog", "Dog": "Animal"})
		},
		want: notFound,
	}, {
		name: "redirecting a schema that does not exist",
		fn: func(d *openapi.Document) error {
			return edit.RedirectSchemas(d, map[string]string{"Cat": "Animal", "Missing": "Animal"})
		},
		want: notFound,
	}, {
		name: "renaming two schemas to one name",
		fn: func(d *openapi.Document) error {
			return edit.RenameSchemas(d, map[string]string{"Cat": "Kitty", "Dog": "Kitty"})
		},
		want: exists,
	}, {
		name: "renaming onto a schema that stays",
		fn: func(d *openapi.Document) error {
			return edit.RenameSchemas(d, map[string]string{"Cat": "Kitty", "Dog": "Bird"})
		},
		want: exists,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d := batchDoc(t)
			before := toJSON(t, d)

			err := tc.fn(d)
			if !tc.want(err) {
				t.Fatalf("unexpected error %T: %v", err, err)
			}

			// A failed batch must change nothing at all.
			if after := toJSON(t, d); after != before {
				t.Error("the document changed despite the error")
			}
		})
	}
}
