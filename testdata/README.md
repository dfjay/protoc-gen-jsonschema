# Test cases

This directory is a separate Go module containing regression cases for the
plugin. Each case compiles input protos with `protoc`, passes the resulting
descriptors to the generator, and compares output byte for byte with committed
expected schemas. Error cases check that generation fails.

Two more checks run on every generated schema:

- `TestPlugin` compiles it, which validates it against the meta-schema of its
  draft and resolves every `$ref`.
- `TestRoundTrip` fills the entrypoint message with random values, writes it
  with `protojson`, sometimes with `EmitDefaultValues`, and validates the JSON
  against the schema. Fields whose annotations or protovalidate rules narrow
  the schema (numeric, string and array keywords, non-string enum mappings,
  object property counts) are left unset, and such messages are written without
  defaults. The schema may still require such a field, where protovalidate
  rejects its zero value; the test accepts errors only about such fields
  missing from the message at the error's location. On failure, [rapid](https://pkg.go.dev/pgregory.net/rapid) prints a
  minimized message and how to rerun it.
- `TestProtovalidate`, for cases with `checkProtovalidate: true`, fills fields
  with values at and around the limits of their protovalidate rules and checks
  that the schema accepts the ProtoJSON of a message exactly when
  [protovalidate-go](https://github.com/bufbuild/protovalidate-go) accepts the
  message, written with and without `EmitDefaultValues`. Such a case uses only rules the schema expresses, and no field
  options that replace them. Candidate values are sorted into those
  protovalidate accepts and rejects, and each field must see both.
- `TestPatterns`, for the same cases, runs each translated protovalidate
  pattern in an ECMA-262 engine ([goja](https://github.com/dop251/goja)) and
  checks it matches the same sample strings as the RE2 original.
- `TestIPFormats`, for the same cases, checks the `ip` and `ipv6` rules, which
  `TestProtovalidate` leaves out with `uri` and `uri_ref`, against protovalidate
  on sample addresses, including IPv6 addresses with a zone.

See [CONTRIBUTING.md](../CONTRIBUTING.md) for prerequisites. Run the following
commands from the repository root:

```sh
just test                      # Both Go modules.
just golden                    # Regenerate all expected schemas.
just golden proto-enum         # Regenerate one case.
```

To run a single case directly:

```sh
cd testdata
go test -run 'TestPlugin/proto-enum$' .
```

The subtest name comes from the case directory, not the YAML `name` field.

## Add a case

1. Create `testdata/cases/<case-name>/` with a `test.proto` and `test.yaml`.
2. Add input messages and select an entrypoint in the proto or plugin options.
3. List inputs and expected output filenames in `test.yaml`.
4. Run `just golden <case-name>`; never write expected schemas by hand.
5. Review the generated schema and run the case, then `just check`.
6. Add a row to the case index below.

For a generator error, set `expectFail: true` and omit expected result files.
The golden generator skips error cases. Inputs must still compile with `protoc`:
this flag checks plugin errors, not invalid protobuf syntax.

For successful generation with no output, use `expectResultIsNull: true`.

### Case configuration

```yaml
name: "example-case"
description: "Describe the behavior under test"
inputFiles: ["test.proto"]
inputParameters:
  entrypoint_message: "Form"
  preserve_proto_field_names: "true"
expectResultFiles: ["test.schema.json"]
expectResultIsNull: false
expectFail: false
```

| Field | Meaning |
| --- | --- |
| `name` | Descriptive identifier used in failure messages. |
| `description` | Human-readable explanation of the case. |
| `inputFiles` | Proto files to compile, relative to the case directory. |
| `inputParameters` | Plugin option names mapped to string values. Quote Boolean and numeric values. |
| `expectResultFiles` | Expected output filenames, relative to the case directory. |
| `expectResultIsNull` | Expect zero output files; defaults to `false`. |
| `expectFail` | Expect a plugin generation error; defaults to `false`. |
| `expectedBehaviorDescription` | Optional descriptive metadata; not asserted by the runner. |
| `skipRoundTrip` | Why `TestRoundTrip` skips the case, when its schema deliberately differs from what `protojson` writes. |
| `checkProtovalidate` | Run `TestProtovalidate` on the case. |

The runner adds the case directory, the repository's `proto` directory, and
`testdata/include` to the include path. `testdata/include` holds third-party
protos the cases import: `buf/validate/validate.proto` from protovalidate
v1.2.2. Golden regeneration compiles every `.proto` in the case directory, so
keep its inputs consistent with `inputFiles`.

## Case index

| Case | Checks |
|------|--------|
| `buf-validate-collections` | protovalidate `repeated` and `map` rules, with `items`, `keys` and `values`; `IGNORE_IF_ZERO_VALUE` exempts empty message elements |
| `buf-validate-enum` | protovalidate `enum` rules restrict the allowed names |
| `buf-validate-enum-lenient` | protovalidate `enum` rules allow names and numbers in Lenient mode |
| `buf-validate-ignore-always` | `IGNORE_ALWAYS` also skips the rules inside the field's message |
| `buf-validate-int64-numbers` | protovalidate 64-bit bounds with `respect_protojson_int64=false` |
| `buf-validate-oneof` | `(buf.validate.oneof).required` and `(buf.validate.message).oneof` |
| `buf-validate-overrides` | field options take precedence over protovalidate rules, also for `required` and wrapped zero values; enum rules on custom values, including objects and values shared by numbers |
| `buf-validate-required` | protovalidate `required`, on fields without presence and oneof members too; rules that reject the zero value of a field without presence make it required |
| `buf-validate-scalars` | protovalidate number, string and bool rules, `ignore`, wrappers, RE2 patterns, bounds narrowing 64-bit `in`, `hostname`, `ipv4` and `uuid` as patterns, and IPv6 zones |
| `buf-validate-scalars-draft-04` | protovalidate scalar rules in draft-04 |
| `buf-validate-scalars-lenient` | protovalidate numeric rules accept only numbers in Lenient mode, within the range of 32-bit types |
| `buf-validate-skipped` | protovalidate rules without a JSON Schema form are left out |
| `jsonschema-array` | array validation keywords |
| `jsonschema-deprecated` | the `deprecated` option on messages, fields and enums becomes `deprecated: true` |
| `jsonschema-deprecated-draft-07` | draft-07 has no `deprecated` keyword, so it is left out |
| `jsonschema-description` | leading, trailing and detached comments become `description` |
| `jsonschema-examples` | `examples` on fields, repeated fields, maps and messages |
| `jsonschema-examples-draft-07` | draft-07 moves a `$ref` with sibling keywords into `allOf` |
| `jsonschema-field-behavior` | `google.api.field_behavior` sets `required`, `readOnly` and `writeOnly` |
| `jsonschema-field-behavior-disabled` | `respect_field_behavior=false` ignores `google.api.field_behavior` |
| `jsonschema-field-required` | the per-field `required` option with `respect_protojson_presence` |
| `jsonschema-nullable` | `nullable` wraps the field schema in a `oneOf` with `null` |
| `jsonschema-number` | numeric validation keywords |
| `jsonschema-number-invalid-multiple-of` | `multiple_of` must be greater than 0 (fails) |
| `jsonschema-object` | object validation keywords |
| `jsonschema-string` | string validation keywords |
| `plugin-additional-properties-always-false` | `additional_properties=AlwaysFalse` sets `additionalProperties: false` on every message |
| `plugin-additional-properties-always-true` | `additional_properties=AlwaysTrue` sets `additionalProperties: true` on every message |
| `plugin-additional-properties-default` | `additional_properties=DefaultAdditionalProperties` behaves like `DoNothing` |
| `plugin-additional-properties-default-false` | `additional_properties=DefaultFalse` applies to messages that don't set it |
| `plugin-additional-properties-default-true` | `additional_properties=DefaultTrue` applies to messages that don't set it |
| `plugin-additional-properties-do-nothing` | `additional_properties=DoNothing` leaves `additionalProperties` alone |
| `plugin-consist-output` | the output is identical across runs, with messages imported from other files |
| `plugin-draft-04` | draft-04 output |
| `plugin-draft-05` | `draft=Draft05` is an unknown draft (fails) |
| `plugin-draft-06` | draft-06 output |
| `plugin-draft-07` | draft-07 output |
| `plugin-draft-201909` | draft-2019-09 output |
| `plugin-draft-202012` | draft-2020-12 output |
| `plugin-enum-value-descriptions` | `enum_value_descriptions` writes one `const` per value with its comment and `deprecated` |
| `plugin-enum-value-descriptions-lenient` | `enum_value_descriptions` in Lenient mode also accepts any integer |
| `plugin-entrypoint` | the `entrypoint_message` plugin option |
| `plugin-entrypoint-file-name-collision` | two entrypoints with the same schema file name are an error (fails) |
| `plugin-entrypoint-message-option` | each message with `entrypoint: true`, nested ones too, gets its own schema file |
| `plugin-entrypoint-message-option-hidden` | a message with `entrypoint: true` hidden by `visibility_level` gets no schema |
| `plugin-entrypoint-message-option-mixed` | `entrypoint_message` and `entrypoint: true` together, with YAML output |
| `plugin-invalid-option` | an unknown option value is rejected (fails) |
| `plugin-invalid-protojson-mode` | an unknown `protojson_mode` is rejected (fails) |
| `plugin-output-yaml` | YAML output via `output_file_suffix` |
| `plugin-preserve-field-name-false` | field names follow `json_name` |
| `plugin-preserve-field-name-true` | field names keep the proto names, ignoring `json_name` |
| `plugin-protojson-mode-lenient` | `protojson_mode=Lenient` accepts what ProtoJSON parsers accept |
| `plugin-respect-buf-validate-false` | `respect_buf_validate=false` ignores protovalidate rules |
| `plugin-respect-protojson-int64-false` | `respect_protojson_int64=false` writes 64-bit integers as integers |
| `plugin-respect-protojson-int64-map` | 64-bit integer map values are strings; the map stays an object |
| `plugin-respect-protojson-no-required` | `respect_protojson_presence` adds no automatic `required` |
| `plugin-respect-protojson-presence-false` | `respect_protojson_presence=false` requires every field that can't be absent |
| `plugin-visibility-level` | fields and messages below `visibility_level` are left out |
| `plugin-visibility-level-hidden-ref` | a visible field of a hidden message type is an error (fails) |
| `proto-circular-dependent` | messages that reference each other |
| `proto-enum` | enums become `enum` |
| `proto-enum-alias` | aliased enum values (`allow_alias`) appear once |
| `proto-import-editions` | a proto3 file uses messages from an edition 2023 file |
| `proto-map` | maps become objects; integer and bool keys get `propertyNames` |
| `proto-message-set` | a MessageSet elsewhere in the file does not stop generation |
| `proto-oneof-multiple` | several oneofs in one message |
| `proto-oneof-single` | a single oneof |
| `proto-optional` | `optional` fields are left out of `required` |
| `proto-version-edition-2023` | `edition = "2023"` |
| `proto-version-edition-2024` | `edition = "2024"` |
| `proto-version-syntax-proto2` | `syntax = "proto2"`, with `optional` and `required` labels |
| `proto-version-syntax-proto3` | `syntax = "proto3"` |
| `wellknown-k8s` | Kubernetes `IntOrString` |
| `wellknown-k8s-entrypoint` | a non-object entrypoint (`IntOrString`) gets no `type: object` at the root |
| `wellknown-proto` | `google.protobuf` well-known types in their ProtoJSON form: `Any`, `Struct`, `Value`, `ListValue`, `NullValue`, `Duration`, `Timestamp`, `FieldMask`, `Empty` and the wrappers |
