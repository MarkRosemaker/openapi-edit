package edit_test

import (
	"errors"
	"testing"

	"github.com/MarkRosemaker/openapi"
	edit "github.com/MarkRosemaker/openapi-edit"
)

const itemRef = "#/components/schemas/Item"

// isItems accepts an array of Item.
func isItems(s *openapi.Schema) bool {
	return s.Type == openapi.TypeArray && s.Items != nil && s.Items.Ref != nil && s.Items.Ref.Identifier == itemRef
}

func itemsDoc() (d *openapi.Document, item, inProp, inResponse *openapi.Schema) {
	item = &openapi.Schema{Type: openapi.TypeString}
	inProp = &openapi.Schema{Description: "The tags.", Type: openapi.TypeArray, Items: ref(itemRef, item)}
	inResponse = &openapi.Schema{Type: openapi.TypeArray, Items: ref(itemRef, item)}

	props := openapi.Schemas{}
	props.Set("tags", inProp)

	op := &openapi.Operation{Responses: openapi.OperationResponses{}}
	op.Responses.Set("200", &openapi.ResponseRef{Value: &openapi.Response{
		Description: "OK",
		Content:     openapi.Content{"application/json": {Schema: inResponse}},
	}})

	d = &openapi.Document{
		OpenAPI: "3.1.0",
		Info:    &openapi.Info{Title: "test", Version: "0.0.0"},
		Paths:   openapi.Paths{"/items": {Get: op}},
	}
	d.Components.Schemas = openapi.Schemas{}
	d.Components.Schemas.Set("Item", item)
	d.Components.Schemas.Set("Holder", &openapi.Schema{Type: openapi.TypeObject, Properties: props})

	return d, item, inProp, inResponse
}

func TestExtractSchema_ReplacesEveryMatch(t *testing.T) {
	d, item, inProp, inResponse := itemsDoc()

	if err := edit.ExtractSchema(d, "Items", isItems); err != nil {
		t.Fatal(err)
	}

	items, ok := d.Components.Schemas["Items"]
	if !ok {
		t.Fatal("Items is not in components.schemas")
	}

	if items.Type != openapi.TypeArray || items.Items.Ref.Value != item || items.Description != "" {
		t.Errorf("Items is %+v, want an array of Item without a description", items)
	}

	for name, s := range map[string]*openapi.Schema{"property": inProp, "response": inResponse} {
		if s.Ref == nil || s.Ref.Identifier != "#/components/schemas/Items" || s.Ref.Value != items {
			t.Errorf("the %s is not a reference to Items", name)
		}

		if s.Type != "" || s.Items != nil {
			t.Errorf("the %s kept its inline definition", name)
		}
	}

	if inProp.Description != "The tags." {
		t.Errorf("the property's description is %q, want it kept on the reference", inProp.Description)
	}
}

func TestExtractSchema_LeavesComponentsAlone(t *testing.T) {
	d, _, _, _ := itemsDoc()

	list := &openapi.Schema{Type: openapi.TypeArray, Items: ref(itemRef, d.Components.Schemas["Item"])}
	d.Components.Schemas.Set("List", list)

	if err := edit.ExtractSchema(d, "Items", isItems); err != nil {
		t.Fatal(err)
	}

	if list.Ref != nil {
		t.Error("a component schema was replaced by a reference")
	}
}

func TestExtractSchema_Fails(t *testing.T) {
	for name, tc := range map[string]struct {
		name  string
		match func(*openapi.Schema) bool
		check func(error) bool
	}{
		"no match": {"Items", func(*openapi.Schema) bool { return false }, func(err error) bool {
			return errors.Is(err, edit.ErrNoMatch)
		}},
		"name taken": {"Item", isItems, func(err error) bool {
			_, ok := errors.AsType[*edit.ErrSchemaExists](err)
			return ok
		}},
		"invalid name": {"Some Items", isItems, func(err error) bool {
			_, ok := errors.AsType[*edit.ErrInvalidSchemaName](err)
			return ok
		}},
	} {
		t.Run(name, func(t *testing.T) {
			d, _, inProp, _ := itemsDoc()

			if err := edit.ExtractSchema(d, tc.name, tc.match); !tc.check(err) {
				t.Fatalf("got error %v", err)
			}

			if inProp.Ref != nil || len(d.Components.Schemas) != 2 {
				t.Error("the document changed")
			}
		})
	}
}
