# protoc-gen-jsonschema

[![CI](https://github.com/dfjay/protoc-gen-jsonschema/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/dfjay/protoc-gen-jsonschema/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/dfjay/protoc-gen-jsonschema)](https://github.com/dfjay/protoc-gen-jsonschema/releases/latest)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

Generate JSON Schema from Protocol Buffer messages with a `protoc` plugin.
Use the schemas to validate JSON or YAML configuration, document an API, or
provide input to tools that consume JSON Schema.

The plugin describes the JSON representation of a message, following
[ProtoJSON](https://protobuf.dev/programming-guides/json/). It supports:

- JSON Schema draft-04, draft-06, draft-07, 2019-09, and 2020-12.
- Protobuf `proto2`, `proto3`, and editions 2023 and 2024.
- JSON and YAML output, custom validation rules, and selected Kubernetes types.

This is a maintained fork of
[pubg/protoc-gen-jsonschema](https://github.com/pubg/protoc-gen-jsonschema).
Releases of this fork continue from `v0.8.0`. Existing users should read
[Migration](#migration).

## Contents

- [Installation](#installation)
- [Quick start](#quick-start)
- [Usage](#usage)
- [Plugin options](#plugin-options)
- [Custom schema options](#custom-schema-options)
- [Protobuf mappings](#protobuf-mappings)
- [Examples and reference](#examples-and-reference)
- [Migration](#migration)
- [Contributing](#contributing)
- [License](#license)

## Installation

Install [protoc](https://protobuf.dev/installation/) and make sure it is on your
`PATH`. Then choose one of the following ways to install the plugin.

### With Go

Use the Go version required by [go.mod](go.mod) or later:

```sh
go install github.com/dfjay/protoc-gen-jsonschema@latest
```

Add Go's binary installation directory (`GOBIN`, or `$(go env GOPATH)/bin` by
default) to your `PATH` so that `protoc` can find `protoc-gen-jsonschema`.
For reproducible builds, replace `@latest` with a release tag.

### Prebuilt binaries

Download an archive for your platform from
[GitHub Releases](https://github.com/dfjay/protoc-gen-jsonschema/releases),
extract it, and put `protoc-gen-jsonschema` on your `PATH`. The archives also
include `proto/dfjay/jsonschema/options.proto` for custom schema annotations.

Release archives have GitHub build attestations. With the GitHub CLI installed,
verify a downloaded archive using:

```sh
gh attestation verify <archive> --repo dfjay/protoc-gen-jsonschema
```

Releases also include checksums, a cosign signature bundle for the checksums,
and software bills of materials.

## Quick start

Create `form.proto`:

```proto
syntax = "proto3";

message Form {
  string display_name = 1;
  int64 account_id = 2;
  repeated string tags = 3;
}
```

Generate a schema for `Form`:

```sh
protoc -I . --jsonschema_out=. \
  --jsonschema_opt=entrypoint_message=Form form.proto
```

This writes `form.schema.json` using JSON Schema 2020-12. It describes an object
with `displayName` (string), `accountId` (decimal string), and `tags` (array of
strings). For example, this JSON is accepted:

```json
{
  "displayName": "Ada",
  "accountId": "9007199254740993",
  "tags": ["admin"]
}
```

Fields may be absent by default. To require a field or add validation rules,
use [custom schema options](#custom-schema-options).

## Usage

### Choose an entrypoint

An entrypoint is a message the plugin writes a schema for. Each schema includes
the types reachable from its entrypoint, including imported types. A file
without an entrypoint produces no schema. There are two ways to choose one.

**One schema per file.** Name a **top-level message in that file** by its
simple name (for example, `Form`, rather than `example.Form`), with
`--jsonschema_opt=entrypoint_message=Form` or with the file annotation shown in
[Custom schema options](#custom-schema-options). A nonempty file annotation
takes precedence over the command-line option; a file without a matching
message produces no schema. `api/form.proto` gives `api/form.schema.json`.

**Any number of schemas per file.** Mark each message, top-level or nested:

```proto
message Customer {
  option (dfjay.jsonschema.message) = {entrypoint: true};
}
```

Each marked message gets its own self-contained schema, named after the file
and the message: `api/form.proto` gives `api/form.Customer.schema.json`, and a
nested `Outer.Inner` gives `api/form.Outer.Inner.schema.json`. Names do not
depend on how many messages are marked, so marking another message renames no
file. Types shared by several entrypoints appear in each schema. A marked
message hidden by `visibility_level` is skipped. Both ways can be combined.
Two entrypoints that get the same file name, such as message `Bar` of
`foo.proto` and the entrypoint of `foo.Bar.proto`, are an error.

Output paths preserve the input path relative to the `protoc` include directory,
under `--jsonschema_out`. Create the output directory before running `protoc`.

### Select a format or draft

```sh
# YAML output.
protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
  --jsonschema_opt=output_file_suffix=.schema.yaml form.proto

# Compact JSON output.
protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
  --jsonschema_opt=pretty_json_output=false form.proto

# JSON Schema draft-07.
protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
  --jsonschema_opt=draft=Draft07 form.proto
```

### Choose a ProtoJSON mode

| Mode | Intended use | Accepted representation |
| --- | --- | --- |
| `Canonical` (default) | Describe protobuf output, document APIs, generate types | Canonical ProtoJSON type mappings, with the nullability rules described below |
| `Lenient` | Validate configuration that a ProtoJSON parser will read | Also permits numeric strings, enum numbers, and other parser-compatible forms |

```sh
protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
  --jsonschema_opt=protojson_mode=Lenient form.proto
```

Lenient mode broadens validation; it does not implement every parser behavior.
Field names still follow `preserve_proto_field_names`: the schema does not accept
both spellings at once. Numeric validation annotations restrict their fields to
numbers. Custom annotations can also intentionally depart from ProtoJSON.

## Plugin options

Pass options as repeated `--jsonschema_opt=name=value` arguments. Enum values are
case-sensitive. Defaults below are the command-line defaults; protobuf field
zero values in the generated reference do not necessarily match them.

| Option | Default | Behavior |
| --- | --- | --- |
| `entrypoint_message` | Unset | Simple name of the top-level message to describe. A nonempty file annotation overrides it. Messages can also be marked with `entrypoint: true`; see [entrypoints](#choose-an-entrypoint). |
| `output_file_suffix` | `.schema.json` | Output name suffix; must end in `.json`, `.yaml`, or `.yml`. |
| `pretty_json_output` | `true` | Pretty-print JSON output. Has no effect on YAML. |
| `draft` | `Draft202012` | One of `Draft04`, `Draft06`, `Draft07`, `Draft201909`, `Draft202012`. |
| `protojson_mode` | `Canonical` | Choose `Canonical` or `Lenient`; see [above](#choose-a-protojson-mode). |
| `preserve_proto_field_names` | `false` | Use proto field names instead of `json_name` or lowerCamelCase. |
| `respect_protojson_presence` | `true` | Add no automatic required fields beyond explicit requirements; see [presence](#presence-and-nullability). |
| `respect_protojson_int64` | `true` | Represent 64-bit integers and their wrappers as decimal strings. Set to `false` for integer values. Ignored in Lenient mode. |
| `mandatory_nullable` | `false` | Set to `true` to disable automatic nullability for optional fields and oneof members in Canonical mode. Explicit `nullable` annotations still apply. |
| `respect_field_behavior` | `true` | Apply `google.api.field_behavior`: `REQUIRED`, `OUTPUT_ONLY`, and `INPUT_ONLY`. |
| `additional_properties` | `DoNothing` | Control unknown object properties; see [below](#additional-properties). |
| `visibility_level` | `0` | Omit messages and fields whose annotated visibility level is below this threshold. A visible reference to a hidden type is an error. |
| `int64_as_string` | `true` | Deprecated alias for `respect_protojson_int64`. If both are supplied, this alias takes precedence. |

`DraftDefault`, `ProtojsonModeDefault`, and `DefaultAdditionalProperties` are
accepted enum sentinel values and resolve to the corresponding plugin defaults.

### Additional properties

| Value | Behavior |
| --- | --- |
| `DoNothing` | Preserve message annotations; otherwise omit `additionalProperties`. |
| `AlwaysTrue` | Set `additionalProperties: true` on every message, overriding annotations. |
| `AlwaysFalse` | Set `additionalProperties: false` on every message, overriding annotations. |
| `DefaultTrue` | Set `additionalProperties: true` where the message has no explicit annotation. |
| `DefaultFalse` | Set `additionalProperties: false` where the message has no explicit annotation. |

When omitted, JSON Schema allows additional properties. Use `DefaultFalse` to
reject unknown fields while allowing individual messages to opt out.

## Custom schema options

The plugin provides annotations for files, messages, fields, enums, and enum
values. Obtain the [`proto`](proto) directory from this repository or a release
archive, and import `dfjay/jsonschema/options.proto`:

```proto
syntax = "proto3";

import "dfjay/jsonschema/options.proto";

option (dfjay.jsonschema.file) = {entrypoint_message: "Form"};

message Form {
  option (dfjay.jsonschema.message) = {
    title: "Registration form"
    object: {additional_properties: false}
  };

  // Public name shown in the application.
  string display_name = 1 [(dfjay.jsonschema.field) = {
    title: "Display name"
    required: true
    string: {min_length: 1, max_length: 80}
  }];

  // Nickname, or null when the user has none.
  string nickname = 2 [(dfjay.jsonschema.field) = {nullable: true}];
}
```

Generate the schema with both your own files and the options directory on the
include path:

```sh
protoc -I . -I path/to/protoc-gen-jsonschema/proto \
  --jsonschema_out=. form.proto
```

Annotations support titles and descriptions, required fields, nullability,
string patterns and lengths, numeric bounds, array constraints, object
constraints, and enum mappings. Explicit descriptions take precedence over
comments; otherwise, descriptor comments become schema descriptions.

See the generated [annotation reference](options.md) for all fields and
[annotation test cases](testdata/cases) for working examples. Some annotation
fields are reserved for incomplete features: in particular, file-level
`visibility_level` is not used by the generator.

## Protobuf mappings

### Scalar and collection types

| Protobuf type | Canonical JSON Schema representation |
| --- | --- |
| `string` | String |
| `bool` | Boolean |
| 32-bit integer types | Integer |
| 64-bit integer types | String with a decimal pattern, unless `respect_protojson_int64=false` |
| `float`, `double` | Number, or one of the strings `NaN`, `Infinity`, `-Infinity` |
| `bytes` | Base64 string with standard alphabet and padding |
| Enum | Enum names by default; annotations can select numbers or custom values |
| Message | Object, except for well-known types listed below |
| `repeated` | Array of the element schema |
| `map` | Object with a schema for its values |

Map keys are strings. Integer and Boolean keys receive a `propertyNames`
constraint in draft-06 and later; draft-04 cannot express this constraint.
In Lenient mode, numeric validation annotations restrict numeric fields to JSON
numbers. In Canonical mode, bounds do not apply to 64-bit integers represented
as strings; set `respect_protojson_int64=false` to use numeric validation for them.

Lenient mode also accepts integers and floats as numbers or strings, enum names
or numbers, URL-safe or unpadded base64, and integer map keys with leading zeros
or a plus sign. It permits `null` for fields, with special handling for repeated
`Value` and `NullValue`. `mandatory_nullable` and `respect_protojson_int64` have
no effect in this mode.

### Presence and nullability

A missing property and a property set to `null` are separate concepts in JSON
Schema. By default, fields may be omitted. The plugin resolves requiredness in
this order, from highest priority to lowest:

1. The field annotation `required: true` or `required: false`.
2. Proto2 `required` or editions `LEGACY_REQUIRED`.
3. `google.api.field_behavior = REQUIRED` on fields outside a oneof, when enabled.
4. With `respect_protojson_presence=false`, fields that are neither optional,
   repeated, maps, nor oneof members become required.

In Canonical mode, optional fields and oneof members accept `null` by default.
Set `mandatory_nullable=true` to disable that automatic behavior. A field
annotation `nullable: true` explicitly permits `null`. Types whose JSON form
already accepts `null`, such as `google.protobuf.Value`, retain that behavior.
Editions use the resolved `field_presence` feature to determine optionality.

Oneofs allow no member to be set, or one member to be set. In Lenient mode, a
member set to `null` normally counts as unset.

`OUTPUT_ONLY` and `INPUT_ONLY` field behaviors become `readOnly` and `writeOnly`
annotations in draft-07 and later. Earlier drafts omit these keywords. These
annotations do not themselves prohibit a property from appearing in input.

### Well-known types

| Type | JSON Schema representation |
| --- | --- |
| `google.protobuf.Timestamp` | String with `format: date-time` |
| `google.protobuf.Duration` | String containing seconds with an `s` suffix, such as `1.5s` |
| `google.protobuf.FieldMask` | String |
| `google.protobuf.Any`, `google.protobuf.Struct` | Object |
| `google.protobuf.ListValue` | Array |
| `google.protobuf.Value` | Any JSON value |
| `google.protobuf.NullValue` | Null |
| Protobuf wrapper types, such as `Int64Value` and `StringValue` | The wrapped value's schema |

The schemas for `Any`, `Struct`, `ListValue`, and `FieldMask` describe their broad
JSON shape rather than validating all protobuf semantics. A validator's handling
of `format`, including `date-time`, depends on its configuration.

### Kubernetes types

The generator has special handling for these fully qualified types:

- `k8s.io.apimachinery.pkg.util.intstr.IntOrString`
- `k8s.io.api.core.v1.Volume`
- `k8s.io.api.core.v1.SecretProjection`
- `k8s.io.api.core.v1.ConfigMapVolumeSource`
- `k8s.io.api.core.v1.ConfigMapProjection`
- `k8s.io.api.core.v1.ConfigMapKeySelector`
- `k8s.io.api.core.v1.SecretKeySelector`
- `k8s.io.api.core.v1.ConfigMapEnvSource`
- `k8s.io.api.core.v1.SecretEnvSource`
- `k8s.io.api.core.v1.Probe`
- `k8s.io.api.core.v1.EphemeralContainer`

## Examples and reference

- [Examples](examples): one input proto with generated JSON and YAML schemas for
  each supported draft.
- [Options reference](options.md): generated documentation for custom annotations.
- [Test cases](testdata/README.md): focused examples and regression coverage.
- [Schema reference files](docs/README.md): JSON Schema meta-schemas in this repository.
- [Changelog](CHANGELOG.md): release history.

## Migration

When migrating from `pubg/protoc-gen-jsonschema`:

1. Install the plugin from `github.com/dfjay/protoc-gen-jsonschema`.
2. Replace the import `jsonschema.proto` with `dfjay/jsonschema/options.proto`.
3. Replace annotation prefixes `(pubg.jsonschema.` with `(dfjay.jsonschema.`.
4. Add the fork's `proto` directory to the `protoc` include path.
5. Regenerate schemas and review the diff, particularly ProtoJSON mappings and
   field presence. Consult the [changelog](CHANGELOG.md) for release-specific changes.

## Contributing

Bug reports and pull requests are welcome. Include a small `.proto` example,
the generation command, and the expected and actual result when reporting a bug.
See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, checks, and the contribution workflow.

## License

[Apache License 2.0](LICENSE).
