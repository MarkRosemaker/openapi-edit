## Scope

Operations belong here when they satisfy two conditions: they **mutate** a
document, and doing them correctly requires knowledge of the document *beyond* the
node being changed.

**In scope**

- ✅ Renaming a component and rewriting every reference to it (`RenameSchema`)
- ✅ Repointing every reference to a duplicate component onto the one that
  survives, and removing the duplicate (`RedirectSchema`)
- Moving a definition between inline and `components`, keeping references intact

**Out of scope**

- Deciding *whether* two things should be merged — that is
  [`openapi-compare`](https://github.com/MarkRosemaker/openapi-compare)
- Combining two independently inferred schemas into one wider schema that
  covers what both described — that is
  [`openapi-merge`](https://github.com/MarkRosemaker/openapi-merge).
  `RedirectSchema` above is a different, narrower operation: it never reads
  or changes either schema's own definition, only the references that
  pointed at the one being discarded.
- Whole-document policies such as flattening or deduplication — those are their own
  modules, and they are expected to *use* this one
- Anything universal enough to belong on the types themselves — that goes into
  [`openapi`](https://github.com/MarkRosemaker/openapi) instead, so that users who
  only want to parse and validate a spec aren't made to carry it

## The openapi family

| Module | Purpose |
|---|---|
| [openapi](https://github.com/MarkRosemaker/openapi) | Parse, validate, and write OpenAPI 3.x specifications |
| [openapi-compare](https://github.com/MarkRosemaker/openapi-compare) | Compare specification objects — exact equality and shape equivalence |
| **openapi-edit** (this module) | Safe structural edits, such as renaming a schema and rewriting every `$ref` to it |
| [openapi-flatten](https://github.com/MarkRosemaker/openapi-flatten) | Promote inline definitions into named `components` entries |
| [openapi-compress](https://github.com/MarkRosemaker/openapi-compress) | Deduplicate and merge equivalent component schemas |
| [openapi-merge](https://github.com/MarkRosemaker/openapi-merge) | Merge schemas that were inferred independently from different samples |
| [openapi-enrich](https://github.com/MarkRosemaker/openapi-enrich) | Infer specification content from observed HTTP traffic |
| [openapi-codegen](https://github.com/MarkRosemaker/openapi-codegen) | Generate Go types, clients, and servers from a specification |
