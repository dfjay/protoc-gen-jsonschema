# protoc-gen-jsonschema

> Maintained fork of [pubg/protoc-gen-jsonschema](https://github.com/pubg/protoc-gen-jsonschema), which is no longer actively developed. Releases continue from `v0.8.0`.
>
> **Migrating from pubg/protoc-gen-jsonschema:** the options file moved from `jsonschema.proto` to `dfjay/jsonschema/options.proto` and its package from `pubg.jsonschema` to `dfjay.jsonschema`. Update the import, replace `(pubg.jsonschema.` with `(dfjay.jsonschema.` and point `-I` at the `proto` directory.

`protoc-gen-jsonschema` is a plugin that converts `protocol buffer` files into `JSON Schema`. While it primarily focuses on the protobuf standard, it also supports certain non-standard specifications, such as those used by Kubernetes, Node.js, and ArgoCD. Rather than trying to cover the entire JSON Schema specification, the plugin is intentionally designed to focus on [ProtoJSON](https://protobuf.dev/programming-guides/json/) and the patterns most frequently used in practice. It supports JSON Schema versions `draft-04`, `draft-06`, `draft-07`, `draft-2019-09`, and `draft-2020-12`, and protocol buffer files in `proto2`, `proto3` and editions `2023` and `2024`. In editions, a field with explicit presence (the `field_presence` feature) is treated like an `optional` field.

If you’d like to support another specification, contributions are always welcome! Feel free to submit a PR.

# Installation

If you have go runtime, you can `go install` it.
```
go install github.com/dfjay/protoc-gen-jsonschema@latest
```

Alternatively, you can download a pre-built binary from [GitHub Release](https://github.com/dfjay/protoc-gen-jsonschema/releases). Releases are signed with cosign and carry build provenance:
```
gh attestation verify protoc-gen-jsonschema_<version>_<os>_<arch>.tar.gz --repo dfjay/protoc-gen-jsonschema
```

# Usage

Refer to the [Plugin Options](#plugin-options) section below for various options available for this plugin.

The plugin writes one schema per `.proto` file, for the message you name as the entrypoint. The schema includes that message and every message, enum and field it uses. Name the entrypoint with `entrypoint_message`, either as a plugin option or in the file (see [below](#im-not-satisfied-with-the-plugins-options-i-want-to-customize-every-field)). Files that do not define their entrypoint message produce no schema.

### Generate a schema
For a `form.proto` with a `message Form`:
```
protoc --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form form.proto
```
This writes `form.schema.json` (draft 2020-12). The examples below assume the entrypoint is set in the file or passed the same way.

### Generate with yaml format
```
protoc --jsonschema_out=. --jsonschema_opt=output_file_suffix=.yaml *.proto
```

### Shrink bytes for transfer over network
```
protoc --jsonschema_out=. --jsonschema_opt=pretty_json_output=false *.proto
```

### I'd like to comply with the protobuf JSON mapping standard
By default the schema follows [ProtoJSON](https://protobuf.dev/programming-guides/json/): it accepts what protobuf writes, including 64-bit integers as strings. If your JSON is not written by protobuf and has 64-bit integers as numbers, use the option below.
```
protoc --jsonschema_out=. --jsonschema_opt=respect_protojson_int64=false *.proto
```

`protojson_mode` chooses between what protobuf writes and what its parser accepts:

| Use                                                                | protojson_mode      |
|--------------------------------------------------------------------|---------------------|
| Generate types, document an API, check protobuf output             | `Canonical` (default) |
| Check JSON or YAML that people write and a protobuf parser reads, such as config files | `Lenient`           |

`Lenient` accepts everything the parser does, such as `null` for any field, numbers in strings and enum numbers, at the cost of looser types (`number | string | null`) for code generators:
```
protoc --jsonschema_out=. --jsonschema_opt=protojson_mode=Lenient *.proto
```

### I'm not satisfied with the plugin's options. I want to customize every field
This plugin offers options for fields, messages, and enums. Import `dfjay/jsonschema/options.proto` (from the [`proto`](proto) directory of this repository, also shipped in every release archive) and set them in your proto:
```proto
import "dfjay/jsonschema/options.proto";

// The message the schema describes; or pass --jsonschema_opt=entrypoint_message=Form.
option (dfjay.jsonschema.file) = {entrypoint_message: "Form"};

message Form {
  string name = 1 [(dfjay.jsonschema.field) = {title: "Name", nullable: true}];
}
```
```
protoc -I path/to/protoc-gen-jsonschema/proto -I . --jsonschema_out=. your.proto
```

# Options

### Plugin Options

### entrypoint_message
```
entrypoint_message names the message the schema describes. It is required: a file whose
entrypoint message is not set, or not defined in that file, produces no schema. The file
option (dfjay.jsonschema.file).entrypoint_message takes precedence over this one.

default: none
example:
    - --jsonschema_opt=entrypoint_message=MyMessage
```

### output_file_suffix
```
output_file_suffix is used to determine output file name suffix.
Values should end with '.json' or '.yaml' or '.yml'.

default: .schema.json
example:
    - --jsonschema_opt=output_file_suffix=.schema.json
    - --jsonschema_opt=output_file_suffix=.schema.yaml
```

### pretty_json_output
```
pretty_json_output is used to determine output json should be pretty printed.
This option is only used when output_file_suffix is '.json'.

default: true
example:
    - --jsonschema_opt=pretty_json_output=true
    - --jsonschema_opt=pretty_json_output=false
```

### draft
```
draft is used to determine which draft version should be used.
The value should be one of Draft04, Draft06, Draft07, Draft201909, Draft202012.

default: Draft202012
example:
    - --jsonschema_opt=draft=Draft202012
```

### mandatory_nullable
```
mandatory_nullable determines whether this plugin should treat optional field as nullable.
Many programming languages do not differentiate between undefined and null.
However, scripting languages like JavaScript and TypeScript can distinguish between them.
By default, optional field is treated as nullable and undefined.

default: false
example:
    - --jsonschema_opt=mandatory_nullable=true
    - --jsonschema_opt=mandatory_nullable=false
```

### int64_as_string
```
Deprecated: use `respect_protojson_int64` instead. It's has same functionality.
Just change the name to be more clear.

Old Description:
int64_as_string determines whether int64 field treat as string.
Depends on Javascript specification, The JS stores integer to only 53bits.
So, if you want to use int64 field in JS, you should use string type.
References:

default: true
example:
    - --jsonschema_opt=int64_as_string=true
    - --jsonschema_opt=int64_as_string=false
```

### preserve_proto_field_names
```
preserve_proto_field_names is used to determine if output json field names
should be identical to the proto field names.
Otherwise field names either use the value of the `json_name` field option
or they are automatically converted to lowerCamelCase.
This default behaviour mirrors the behaviour of Protobuf's canonical JSON format (ProtoJSON).

default: false
example:
    - --jsonschema_opt=preserve_proto_field_names=true
    - --jsonschema_opt=preserve_proto_field_names=false
```

### additional_properties
```
additional_properties option can controls all message's additional_properties property.
If you want set additional properties for all messages, use always_true or always_false.
If you want to set additional properties for not defined messages, use default_true or default_false.

default: 'DoNothing'
example:
  - --jsonschema_opt=additional_properties=AlwaysTrue
  - --jsonschema_opt=additional_properties=AlwaysFalse
  - --jsonschema_opt=additional_properties=DefaultTrue
  - --jsonschema_opt=additional_properties=DefaultFalse
  - --jsonschema_opt=additional_properties=DoNothing
```

### respect_protojson_presence
```
respect_protojson_presence follows the ProtoJSON rule that a parser accepts a message
with any field absent. If set to true, the plugin adds to `required` only fields that
must be present anyway: fields with the `required` field option set to true, proto2
`required` and editions `LEGACY_REQUIRED` fields, and fields with the REQUIRED
`google.api.field_behavior`. If set to false, every field that is not optional, repeated,
a map or a oneof member is required as well.

default: true
example:
  - --jsonschema_opt=respect_protojson_presence=true
  - --jsonschema_opt=respect_protojson_presence=false
```

### respect_protojson_int64
```
respect_protojson_int64 writes 64-bit integer fields (int64, uint64, sint64, fixed64,
sfixed64 and their wrappers) as strings, as ProtoJSON does. If set to false, they are
integers, for JSON readers that parse large numbers themselves; JavaScript numbers lose
precision above 2^53.

default: true
example:
  - --jsonschema_opt=respect_protojson_int64=true
  - --jsonschema_opt=respect_protojson_int64=false
```

### protojson_mode
```
protojson_mode chooses which JSON the schema accepts.
Canonical accepts what protobuf's ProtoJSON serializer writes.
Lenient also accepts what ProtoJSON parsers accept: null for any field, integers and
floats as numbers or strings, enum values as names or numbers, URL-safe and unpadded
base64, and integer map keys with leading zeros or a plus sign. Fields with numeric
keywords accept only numbers. In Lenient, mandatory_nullable and respect_protojson_int64
have no effect; respect_protojson_presence still decides which fields are required.
Field names stay as configured: parsers also accept the proto field name, but
the schema does not.

Use Canonical to generate types, document an API or check protobuf output; use Lenient
to check JSON or YAML that people write and a protobuf parser reads, such as config files.

default: Canonical
example:
  - --jsonschema_opt=protojson_mode=Canonical
  - --jsonschema_opt=protojson_mode=Lenient
```

### respect_field_behavior
```
respect_field_behavior applies the `google.api.field_behavior` field option
(google/api/field_behavior.proto): a REQUIRED field is required, an OUTPUT_ONLY field is
`readOnly` and an INPUT_ONLY field is `writeOnly`. `readOnly` and `writeOnly` need
draft-07 or later. The `required` field option takes precedence over REQUIRED.

default: true
example:
  - --jsonschema_opt=respect_field_behavior=true
  - --jsonschema_opt=respect_field_behavior=false
```

### Protobuf Options

Below tables are not auto-generated features.
Check out the [Options](./options.md) file to see auto-generated options.

| protobuf label | jsonschema                           |
|----------------|--------------------------------------|
| required       | $.required.append(field)             |
| optional       | oneof {type: null, $original_schema} |
| repeated       | type: array, items: $original_schema |

Protobuf well-known types follow their ProtoJSON form:

| Well-known type                                       | jsonschema                                       |
|-------------------------------------------------------|--------------------------------------------------|
| google.protobuf.Timestamp                             | type: string, format: date-time                  |
| google.protobuf.Duration                              | type: string, pattern: seconds with suffix `s`   |
| google.protobuf.FieldMask                             | type: string                                     |
| google.protobuf.Any, google.protobuf.Struct           | type: object                                     |
| google.protobuf.ListValue                             | type: array                                      |
| google.protobuf.Value                                 | any value                                        |
| google.protobuf.NullValue                             | type: null                                       |
| google.protobuf wrappers (Int32Value, StringValue...) | the schema of the wrapped value                  |

Map keys are strings in ProtoJSON: integer and bool keys get a `propertyNames` schema
(draft-06 or later).

The plugin also has schemas for these Kubernetes types:

| Kubernetes types                                |
|-------------------------------------------------|
| k8s.io.apimachinery.pkg.util.intstr.IntOrString |
| k8s.io.api.core.v1.Volume                       |
| k8s.io.api.core.v1.SecretProjection             |
| k8s.io.api.core.v1.ConfigMapVolumeSource        |
| k8s.io.api.core.v1.ConfigMapProjection          |
| k8s.io.api.core.v1.ConfigMapKeySelector         |
| k8s.io.api.core.v1.SecretKeySelector            |
| k8s.io.api.core.v1.ConfigMapEnvSource           |
| k8s.io.api.core.v1.SecretEnvSource              |
| k8s.io.api.core.v1.Probe                        |
| k8s.io.api.core.v1.EphemeralContainer           |

If you'd like to contribute well-known types, Please check [the generator](./pkg/modules/1_frontend_generator.go) file.

# Output Examples
You can find basic example cases in the `./examples` directory and more complex cases in `./testdata/cases.`

# Development Environment
You should install `protoc` and `protoc-gen-go` to your development environment. Below are required tool versions.
- protoc: 32.0
- protoc-gen-go: 1.36.7
- go: 1.24.3
- protoc-gen-doc: latest
