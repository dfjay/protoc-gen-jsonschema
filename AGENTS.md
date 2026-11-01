# AGENTS.md

`protoc-gen-jsonschema`: a `protoc` plugin that writes a self-contained JSON Schema
(draft-04 to 2020-12, JSON or YAML) of the ProtoJSON form of each entrypoint message.
User-facing usage is in `README.md`; contributor setup is in `CONTRIBUTING.md`.
This file records implementation constraints for coding agents.

## Commands

The `justfile` is the source of truth for local checks. CI checks both modules,
including builds and tests on the minimum supported and latest stable Go versions.

```sh
just check          # lint + test: run before you finish
just test           # go test in the root and testdata modules
just lint           # gofmt, go vet, golangci-lint in both modules
just golden [case]  # rewrite expected schemas in testdata/cases
just generate       # regenerate pkg/proto/options.pb.go and options.md
just examples       # regenerate examples/
just dump-descriptors <case>   # descriptor set of a case into .bin/ for debugging
cd testdata && go test -run 'TestPlugin/proto-enum$' .   # one case
```

Tests and `just golden` shell out to `protoc` (CI pins its version in
`.github/workflows/ci.yml`); `just golden` also needs `yq`.

## Done means

- `just check` passes.
- If `pkg/` changed: run `just golden`, then read the diff of every changed expected
  schema and say why each change is intended. A refactor leaves all of them
  byte-identical.
- New behavior has a case in `testdata/cases/<name>/` and a row in the table in
  `testdata/README.md`.
- Docs match the change: `README.md` for options and mappings, `CONTRIBUTING.md`
  for contributor workflows, `options.md` and
  `examples/` regenerated rather than edited.

## Where things are

- `pkg/modules/` is the pipeline; the numeric file prefixes give the stage order
  (`0_module.go` entry → `1_frontend_*` build per-descriptor schemas → `2_` prune to
  what the entrypoint reaches → `3_` convert to the draft → `4_` serialize).
- The frontend visits every file in the request, imports included. Schemas are keyed
  by fully qualified name with a leading dot: `.pkg.Message.field`.
- `pkg/jsonschema/schema.go` is the draft-neutral model; `pkg/jsonschema/draft_*/` map it
  to each draft. A new keyword needs the field, its line in `DeepCopy`, and a mapping in
  every `draft_*` package that has the keyword.
- `testdata/` is a separate Go module (`replace` to `../`): test-only dependencies go in
  `testdata/go.mod`, and lint/test must run in both modules.

## Rules

- Each output file is a self-contained schema of one entrypoint: no `$ref` to other
  files. Output names must not depend on how many entrypoints a file has
  (`form.schema.json` for `entrypoint_message`, `form.<Message>.schema.json` for
  `entrypoint: true`), so adding an entrypoint renames no file.
- Output must be deterministic: same input, byte-identical schema. Use
  `jsonschema.OrderedMap`; never range over a Go map into output.
- Output must be valid for the selected draft. Don't emit keywords a draft lacks
  (`readOnly`/`writeOnly` need draft-07+, `propertyNames` draft-06+). Draft-07 and
  earlier ignore siblings of `$ref`: put the ref in `allOf` before adding keywords.
- Follow ProtoJSON. `protojson_mode=Canonical` (default) describes what protobuf writes;
  `Lenient` also accepts what its parsers accept. A change to a type mapping must handle
  both modes. In Canonical: `json_name`/lowerCamelCase names unless
  `preserve_proto_field_names`; 64-bit integers as strings with a decimal `pattern`, not
  `format: int64`; float NaN/Infinity as strings; map keys are strings; well-known types
  use their JSON form.
- Required precedence, highest first: `(dfjay.jsonschema.field).required`, proto2
  `required` / editions `LEGACY_REQUIRED`, `google.api.field_behavior` REQUIRED, then
  `respect_protojson_presence` (default true: nothing else is required).
- Editions: take presence from the resolved `field_presence` feature (`isOptionalField`,
  `fieldPresence`); groups and DELIMITED fields are messages. When changing the supported
  editions in `Generate`, update `TestSupportedEditions`.
- Use `protobuf-go` directly (`pluginpb`, `protodesc`, `protoreflect`). Don't use
  `protogen` (it requires `go_package` on every file) or `protoc-gen-star`.
- Don't link googleapis: `google.api.field_behavior` is read from the request descriptors.
- Report errors in `CodeGeneratorResponse.Error`, wrapped with the file path; `main.go`
  exits non-zero only when it cannot talk to `protoc`. Log through `logger` (stderr).
- A new plugin option needs: a documented field in `PluginOptions` in
  `proto/dfjay/jsonschema/options.proto`, `just generate`, parsing with its default in
  `pkg/proto/options.go`, a `plugin-*` case, and an entry in the `README.md` plugin options table. Reject unknown
  enum names and malformed values with an error.
- `options.proto` is public API: never renumber or reuse field numbers; keep extension
  numbers 11341–11345. Never edit `pkg/proto/options.pb.go` or `options.md` by hand.
- Never hand-write expected schemas in `testdata/cases`; `just golden` writes them.
- The code uses Go 1.26: `new(expr)` (`new(true)`, `new(int(x))`) is valid; don't
  rewrite it. Also range-over-func iterators (`listOf`) and `slices`/`maps`/`cmp`.
- Don't add root-module dependencies without asking.
- Comments say why, briefly.

## Commits and releases

- Don't commit, push or open a PR unless asked.
- Conventional Commits (`feat:`, `fix:`, `refactor:`, `chore:`; `!` for breaking). PRs
  are squash-merged with the PR title as the commit title, so the title is the changelog
  entry. Put `BREAKING CHANGE:` details in the PR body; an extra conventional line at
  the bottom of the body adds a second changelog entry. Pre-1.0, breaking bumps minor.
- release-please owns `CHANGELOG.md` and `.release-please-manifest.json`; don't edit them.
