package edit

import (
	"maps"
	"slices"

	"github.com/MarkRosemaker/openapi"
)

// SchemaRef is a reference to the component schema name, as a schema to put where the component is wanted.
func SchemaRef(doc *openapi.Document, name string) *openapi.Schema {
	return &openapi.Schema{Ref: &openapi.SchemaRef{Identifier: schemaRefPrefix + name, Value: doc.Components.Schemas[name]}}
}

// MoveToComponents moves s, a schema written out inline, into components.schemas as name, and puts a reference to it
// in its place: a way to name what a document spells out where it uses it. It returns the moved schema.
//
// It moves nothing, and reports false, if s is a reference already, or name is taken or could not be referenced:
// so that a pass naming many schemas can offer each a name and leave alone what has one.
func MoveToComponents(doc *openapi.Document, s *openapi.Schema, name string) (*openapi.Schema, bool) {
	if s.Ref != nil || doc.Components.Schemas[name] != nil || !reComponentKey.MatchString(name) {
		return nil, false
	}

	moved := new(openapi.Schema)
	moved.Replace(s)
	doc.Components.Schemas.Set(name, moved)
	s.Replace(SchemaRef(doc, name))

	return moved, true
}

// RenameSchemasFunc renames each component schema to what rename makes of its name, as [RenameSchemas] does, such as to
// drop a suffix every name carries. A name rename leaves as it is stays.
//
// It fails, changing nothing, for the reasons RenameSchemas does.
func RenameSchemasFunc(doc *openapi.Document, rename func(string) string) error {
	to := map[string]string{}

	for name := range doc.Components.Schemas.ByIndex() {
		if newName := rename(name); newName != name {
			to[name] = newName
		}
	}

	return RenameSchemas(doc, to)
}

// MergeSchemas puts merged in components.schemas as name in place of the schemas members, and points every reference
// to one of them at it, as [RedirectSchemas] does: the step after working out the one schema that allows what several
// near-duplicates did. name may be one of members, whose place it then takes; otherwise it must be free.
//
// It fails, changing nothing, if a member is not in components.schemas ([ErrSchemaNotFound]), or name is taken by a
// schema that is no member ([ErrSchemaExists]) or could not be referenced ([ErrInvalidSchemaName]).
func MergeSchemas(doc *openapi.Document, name string, merged *openapi.Schema, members ...string) error {
	schemas := doc.Components.Schemas

	for _, m := range members {
		if _, ok := schemas[m]; !ok {
			return &ErrSchemaNotFound{Name: m}
		}
	}

	switch {
	case !reComponentKey.MatchString(name):
		return &ErrInvalidSchemaName{Name: name}
	case schemas[name] != nil && !slices.Contains(members, name):
		return &ErrSchemaExists{Name: name}
	}

	to := make(map[string]string, len(members))
	for _, m := range members {
		to[m] = name
	}

	// a member of the name keeps its place, with merged in it, and the others are redirected to it
	schemas.Set(name, merged)
	delete(to, name)

	// the references to name itself, if a member, now find merged
	WalkSchemas(doc, func(s *openapi.Schema) {
		if s.Ref != nil && s.Ref.Identifier == schemaRefPrefix+name {
			s.Ref.Value = merged
		}
	})

	if len(to) == 0 {
		return nil
	}

	return RedirectSchemas(doc, maps.Clone(to))
}
