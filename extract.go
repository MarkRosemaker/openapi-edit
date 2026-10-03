package edit

import (
	"errors"

	"github.com/MarkRosemaker/openapi"
)

// ErrNoMatch is returned when no inline schema matches, so there is nothing to extract.
var ErrNoMatch = errors.New("no inline schema matches")

// ExtractSchema moves the inline schemas match accepts into components.schemas, as one schema called name, and
// replaces each with a reference to it.
//
// The first schema match accepts becomes the component, so match should accept only schemas that are the same:
// every one it accepts is replaced, and what set the others apart is lost. Only their descriptions are kept, on the
// references that replace them, since a description says what a schema is used for there, not what it is.
//
// The schemas already in components.schemas are not inline, so match is never asked about them. It is asked about
// schemas within them, and anywhere else in the document.
//
// It fails, changing nothing, if name is already taken ([ErrSchemaExists]), could not be referenced
// ([ErrInvalidSchemaName]), or if match accepts no schema ([ErrNoMatch]).
func ExtractSchema(doc *openapi.Document, name string, match func(*openapi.Schema) bool) error {
	if !reComponentKey.MatchString(name) {
		return &ErrInvalidSchemaName{Name: name}
	}

	if _, ok := doc.Components.Schemas[name]; ok {
		return &ErrSchemaExists{Name: name}
	}

	named := map[*openapi.Schema]bool{}
	for _, s := range doc.Components.Schemas {
		named[s] = true
	}

	var extracted *openapi.Schema

	walkSchemas(doc, func(s *openapi.Schema) {
		// the walk reaches the extracted schema through the references it leaves behind
		if s.Ref != nil || named[s] || s == extracted || !match(s) {
			return
		}

		if extracted == nil {
			extracted = new(openapi.Schema)
			extracted.Replace(s)
			extracted.Description = ""
		}

		s.Replace(&openapi.Schema{
			Description: s.Description,
			Ref:         &openapi.SchemaRef{Identifier: schemaRefPrefix + name, Value: extracted},
		})
	})

	if extracted == nil {
		return ErrNoMatch
	}

	doc.Components.Schemas.Set(name, extracted)

	return nil
}
