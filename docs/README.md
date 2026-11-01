# JSON Schema reference files

This directory stores JSON Schema meta-schemas: schemas that describe the
structure of other schemas. These files are reference material, not generated
output from the plugin.

| File | Draft |
| --- | --- |
| [schema-draft04.json](schema-draft04.json) | draft-04 |
| [schema-draft06.json](schema-draft06.json) | draft-06 |
| [schema-draft07.json](schema-draft07.json) | draft-07 |
| [schema-draft201909.json](schema-draft201909.json) | 2019-09 |
| [schema-draft202012.json](schema-draft202012.json) | 2020-12 |
| [validation-schema-201909.json](validation-schema-201909.json) | 2019-09 validation vocabulary |

Later drafts split their meta-schemas into multiple referenced documents. These
local files are not a complete offline bundle of those references.

For plugin usage, see the [project README](../README.md). For generated output,
see [examples](../examples). The `$schema` URI in an output document identifies
its draft; `$id` identifies a schema, while `title` and `description` provide
human-readable annotations. Draft-04 uses `id` instead of `$id`.
