# Test cases

This directory is a separate Go module containing regression cases for the
plugin. Each case compiles input protos with `protoc`, passes the resulting
descriptors to the generator, and compares output byte for byte with committed
expected schemas. Error cases check that generation fails.

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

The runner adds the case directory and the repository's `proto` directory to
the include path. Golden regeneration compiles every `.proto` in the case
directory, so keep its inputs consistent with `inputFiles`.

## Case index

| Case | Checks |
|------|--------|
| `jsonschema-array` | array validation keywords |
| `jsonschema-description` | leading, trailing and detached comments become `description` |
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
| `plugin-entrypoint` | the `entrypoint_message` plugin option |
| `plugin-entrypoint-message-option` | each message with `entrypoint: true`, nested ones too, gets its own schema file |
| `plugin-entrypoint-message-option-hidden` | a message with `entrypoint: true` hidden by `visibility_level` gets no schema |
| `plugin-entrypoint-message-option-mixed` | `entrypoint_message` and `entrypoint: true` together, with YAML output |
| `plugin-invalid-option` | an unknown option value is rejected (fails) |
| `plugin-invalid-protojson-mode` | an unknown `protojson_mode` is rejected (fails) |
| `plugin-output-yaml` | YAML output via `output_file_suffix` |
| `plugin-preserve-field-name-false` | field names follow `json_name` |
| `plugin-preserve-field-name-true` | field names keep the proto names, ignoring `json_name` |
| `plugin-protojson-mode-lenient` | `protojson_mode=Lenient` accepts what ProtoJSON parsers accept |
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
| `wellknown-proto` | `google.protobuf` well-known types in their ProtoJSON form: `Any`, `Struct`, `Value`, `ListValue`, `NullValue`, `Duration`, `Timestamp`, `FieldMask`, `Empty` and the wrappers |
