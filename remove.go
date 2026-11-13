package edit

import (
	"encoding/json/v2"
	"maps"
	"slices"
	"strings"

	"github.com/MarkRosemaker/openapi"
)

// RemoveNullAlternatives takes null out of every anyOf, and the anyOf with it where one alternative is left, which
// then takes its place, keeping the title, description and default the anyOf had. FastAPI, for one, writes every
// optional value as an anyOf of its type and null, where what the API means is a value that may be left out.
//
// A reference left alone gets the anyOf's example, if what it refers to has none.
func RemoveNullAlternatives(doc *openapi.Document) {
	WalkSchemas(doc, removeNull)
}

func removeNull(s *openapi.Schema) {
	alts := s.AnyOf[:0]
	for _, alt := range s.AnyOf {
		if alt.Type != openapi.TypeNull {
			alts = append(alts, alt)
		}
	}

	if len(alts) == len(s.AnyOf) {
		return
	}

	if s.AnyOf = alts; len(alts) != 1 {
		return
	}

	only := alts[0]
	if only.Ref != nil {
		if only.Ref.Value != nil && only.Ref.Value.Example == nil {
			only.Ref.Value.Example = s.Example
		}

		s.Replace(&openapi.Schema{Ref: only.Ref, Title: s.Title, Description: s.Description, Default: s.Default})

		return
	}

	title, description, def := s.Title, s.Description, s.Default
	s.Replace(only)
	s.Title, s.Description = title, description

	if len(def) > 0 {
		s.Default = def
	}
}

// RemoveEmptyJSON removes, from every response that has another media type, the JSON whose schema says nothing of
// the value: FastAPI, for one, declares an empty JSON beside the media type a download really has, such as a ZIP or a
// GIF, which would leave a client unable to read the download. A media type without a schema, as a download's often
// is, is no empty JSON.
func RemoveEmptyJSON(doc *openapi.Document) {
	for _, item := range doc.Paths.ByIndex() {
		for _, op := range item.Operations {
			for _, rsp := range op.Responses.ByIndex() {
				if rsp.Value == nil {
					continue
				}

				content := rsp.Value.Content
				if !slices.ContainsFunc(slices.Collect(maps.Keys(content)), func(mr openapi.MediaRange) bool {
					return !emptyJSON(mr, content[mr])
				}) {
					continue
				}

				maps.DeleteFunc(content, emptyJSON)
			}
		}
	}
}

// emptyJSON reports whether mt, of the media range mr, is JSON of a schema of no keywords.
func emptyJSON(mr openapi.MediaRange, mt *openapi.MediaType) bool {
	if mt == nil || mt.Schema == nil {
		return false
	}

	media := strings.TrimSpace(strings.SplitN(string(mr), ";", 2)[0])
	if media != "application/json" && !strings.HasSuffix(media, "+json") {
		return false
	}

	b, err := json.Marshal(mt.Schema)

	return err == nil && string(b) == "{}"
}
