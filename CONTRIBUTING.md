# Contributing

Thanks for helping improve `protoc-gen-jsonschema`. Small, focused changes with
reproducible examples are easiest to review.

## Report a problem

Open a [GitHub issue](https://github.com/dfjay/protoc-gen-jsonschema/issues) with:

- The plugin and `protoc` versions.
- A minimal `.proto` file and any imported custom options.
- The command and plugin options you used.
- The generated schema and the behavior you expected.

For feature requests, describe the JSON you need to represent and how you plan
to use the schema.

## Set up a development environment

Clone the repository and install:

- Go at the version in [go.mod](go.mod) or later.
- `protoc`; use the version pinned in [CI](.github/workflows/ci.yml) to match CI.
- [just](https://github.com/casey/just) to run project tasks.
- [golangci-lint](https://golangci-lint.run/) v2; CI pins its exact version.
- [Mike Farah's yq](https://github.com/mikefarah/yq) v4 to regenerate test fixtures.

Run commands from the repository root unless noted otherwise:

```sh
just          # List available tasks.
just check    # Format check, vet, lint, and tests in both Go modules.
```

Tests invoke `protoc`. The `testdata` directory is a separate Go module, so a
root-level `go test ./...` does not run the fixture suite. `just test` runs both.

Third-party protos that test cases import live in `testdata/include`, which
every `protoc` invocation adds to the include path. To update
`buf/validate/validate.proto`, copy it from a
[protovalidate release](https://github.com/bufbuild/protovalidate/releases),
update the version named in the [test guide](testdata/README.md), and bump
`buf.build/go/protovalidate` in `testdata/go.mod` to a release built on it:
`TestProtovalidate` checks the schemas against that library.

## Make a change

1. Keep changes focused on one behavior or problem.
2. Add a regression case for new behavior; follow the [test guide](testdata/README.md).
3. Update the README when options, mappings, or user-visible behavior change.
4. Run the relevant generators and review their output.
5. Run `just check` before submitting a pull request.

The [justfile](justfile) defines the supported commands:

| Command | Purpose |
| --- | --- |
| `just test` | Run tests in both modules. |
| `just lint` | Check formatting, run vet, and run golangci-lint in both modules. |
| `just golden [case]` | Regenerate all expected schemas, or one named case. |
| `just generate` | Generate Go option bindings and `options.md` from `options.proto`. |
| `just examples` | Regenerate the schemas in `examples/`. |
| `just dump-descriptors <case>` | Write a case's descriptor set under `.bin/descriptors/`. |

`just generate` builds `protoc-gen-go` at the version pinned by `go.mod` and
installs the pinned `protoc-gen-doc` into `.bin/`. It may need network access.

Do not edit `options.md`, `pkg/proto/options.pb.go`, generated examples, or
expected schemas by hand. Edit their sources and regenerate them. For changes
under `pkg/`, run `just golden` and inspect every changed schema; a refactor
should leave the fixtures byte-identical.

## Code layout and conventions

- `main.go` handles communication with `protoc`.
- `pkg/modules/` implements the pipeline: descriptor traversal, schema building,
  pruning, draft conversion, and serialization. Numeric filename prefixes show
  the stage order.
- `pkg/jsonschema/` contains the draft-neutral model and draft-specific mappings.
- `proto/dfjay/jsonschema/options.proto` defines the public annotation API.
- `testdata/cases/` contains input protos, case configuration, and generated fixtures.

Keep output deterministic and valid for the selected draft. Changes to protobuf
mappings must consider both Canonical and Lenient modes. Preserve public option
field numbers and extension numbers. Ask before adding root-module dependencies;
test-only dependencies belong in `testdata/go.mod`.

[AGENTS.md](AGENTS.md) records additional implementation constraints for coding agents.

## Pull requests and releases

Use a Conventional Commit title, such as `fix: preserve optional field presence`
or `feat: support a new schema option`. Pull requests are squash-merged, and the
title becomes the changelog entry. For breaking changes, add `!` to the title
and explain the impact and migration in a `BREAKING CHANGE:` section of the body.
Before 1.0, breaking changes bump the minor version.

Describe the problem, the resulting behavior, and how you verified it. Include
an explanation of intentional changes to generated schemas.

Release automation owns `CHANGELOG.md` and `.release-please-manifest.json`;
leave these files to release-please.
