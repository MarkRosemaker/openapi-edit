package edit_test

import (
	"errors"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

// redirectDoc builds a document with two component schemas, "Old" and "New",
// and a "Parent" schema whose "child" property points at "Old".
func redirectDoc() (d *openapi.Document, child *openapi.SchemaRef) {
	d = &openapi.Document{
		OpenAPI: "3.1.0",
		Info:    &openapi.Info{Title: "test", Version: "0.0.0"},
	}
	d.Components.Schemas = openapi.Schemas{}
	d.Components.Schemas.Set("Old", &openapi.Schema{Type: openapi.TypeObject})
	d.Components.Schemas.Set("New", &openapi.Schema{Type: openapi.TypeString})

	child = ref(oldRef, d.Components.Schemas["Old"])
	parent := &openapi.Schema{Type: openapi.TypeObject, Properties: openapi.SchemaRefs{}}
	parent.Properties.Set("child", child)
	d.Components.Schemas.Set("Parent", parent)

	return d, child
}

func TestRedirectSchema_RepointsRefsAndDeletesOld(t *testing.T) {
	d, child := redirectDoc()
	newSchema := d.Components.Schemas["New"]

	if err := edit.RedirectSchema(d, "Old", "New", ""); err != nil {
		t.Fatal(err)
	}

	if _, ok := d.Components.Schemas["Old"]; ok {
		t.Error("the old schema is still present")
	}

	if got := d.Components.Schemas["New"]; got != newSchema {
		t.Error("the new schema's own definition was disturbed")
	}

	if child.Ref.Identifier != newRef {
		t.Errorf("reference = %q, want %q", child.Ref.Identifier, newRef)
	}
}

func TestRedirectSchema_SetsDescriptionOnRepointedRefs(t *testing.T) {
	d, child := redirectDoc()

	if err := edit.RedirectSchema(d, "Old", "New", "was Old"); err != nil {
		t.Fatal(err)
	}

	if child.Ref.Description != "was Old" {
		t.Errorf("description = %q, want %q", child.Ref.Description, "was Old")
	}
}

func TestRedirectSchema_EmptyDescriptionLeavesExistingOneAlone(t *testing.T) {
	d, child := redirectDoc()
	child.Ref.Description = "already set"

	if err := edit.RedirectSchema(d, "Old", "New", ""); err != nil {
		t.Fatal(err)
	}

	if child.Ref.Description != "already set" {
		t.Errorf("description = %q, want it left alone", child.Ref.Description)
	}
}

func TestRedirectSchema_LeavesOtherRefsAlone(t *testing.T) {
	const otherRef = "#/components/schemas/Other"

	d, _ := redirectDoc()
	other := ref(otherRef, &openapi.Schema{Type: openapi.TypeBoolean})
	d.Components.Schemas["Parent"].Properties.Set("other", other)

	if err := edit.RedirectSchema(d, "Old", "New", ""); err != nil {
		t.Fatal(err)
	}

	if other.Ref.Identifier != otherRef {
		t.Errorf("unrelated reference changed to %q", other.Ref.Identifier)
	}
}

func TestRedirectSchema_LeavesInlineSchemasAlone(t *testing.T) {
	d, _ := redirectDoc()
	inline := &openapi.SchemaRef{Value: &openapi.Schema{Type: openapi.TypeObject}}
	d.Components.Schemas["Parent"].Properties.Set("inline", inline)

	if err := edit.RedirectSchema(d, "Old", "New", "description"); err != nil {
		t.Fatal(err)
	}

	if inline.Ref != nil {
		t.Errorf("an inline schema grew a $ref: %+v", inline.Ref)
	}
}

func TestRedirectSchema_SameName(t *testing.T) {
	d, child := redirectDoc()

	if err := edit.RedirectSchema(d, "Old", "Old", "description"); err != nil {
		t.Fatalf("redirecting a schema onto itself should do nothing, got %v", err)
	}

	if _, ok := d.Components.Schemas["Old"]; !ok {
		t.Error("the schema was removed")
	}

	if child.Ref.Identifier != oldRef {
		t.Error("the reference was disturbed")
	}

	if child.Ref.Description != "" {
		t.Error("the description was set despite the no-op")
	}
}

func TestRedirectSchema_Errors(t *testing.T) {
	notFound := func(err error) bool {
		var e *edit.ErrSchemaNotFound
		return errors.As(err, &e)
	}

	for _, tc := range []struct {
		name     string
		old, new string
	}{
		{name: "the old schema does not exist", old: "Missing", new: "New"},
		{name: "the new schema does not exist", old: "Old", new: "Missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, child := redirectDoc()

			err := edit.RedirectSchema(d, tc.old, tc.new, "")
			if err == nil {
				t.Fatal("expected an error")
			}

			if !notFound(err) {
				t.Errorf("unexpected error type %T: %v", err, err)
			}

			// A failed redirect must change nothing at all.
			if _, ok := d.Components.Schemas["Old"]; !ok {
				t.Error("the old schema was removed despite the error")
			}

			if child.Ref.Identifier != oldRef {
				t.Error("the reference was rewritten despite the error")
			}
		})
	}
}
