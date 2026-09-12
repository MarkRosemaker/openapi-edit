Renaming a schema is the canonical example. The rename itself is a single map
operation, but every `$ref` that pointed at the old name is now dangling — and those
`$ref`s can be anywhere: nested inside another schema's properties, inside an
`allOf` branch, in a response's content, in a parameter, in a callback. Getting this
right means walking the entire document. Getting it wrong means a spec that looks
fine and fails to resolve.

That traversal is worth writing once, carefully, and reusing.

This module serves two kinds of caller:

- **Directly**, when you are writing code against your own specification and want to
  make a specific change safely, without reimplementing the bookkeeping.
- **As a dependency**, for tools like
  [`openapi-compress`](https://github.com/MarkRosemaker/openapi-compress) and
  [`openapi-flatten`](https://github.com/MarkRosemaker/openapi-flatten) that run an
  algorithm over a whole specification and need the same primitives underneath.
