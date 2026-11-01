package modules

import (
	"encoding/json"
	"iter"
	"reflect"
	"slices"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func buildFromMessage(pluginOptions *proto.PluginOptions, behaviors fieldBehaviors, message protoreflect.MessageDescriptor, mo *proto.MessageOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Type = "object"
	schema.Title = proto.GetTitleOrEmpty(mo)
	schema.Description = proto.GetDescriptionOrComment(message, mo)
	schema.Properties = jsonschema.NewOrderedSchemaMap()

	fillSchemaByObjectKeywords(pluginOptions, schema, mo.GetObject())

	for field := range listOf(message.Fields()) {
		if isHiddenField(pluginOptions, field) {
			continue
		}

		propName := toPropertyName(field, pluginOptions.GetPreserveProtoFieldNames())
		fieldSchema := &jsonschema.Schema{Ref: toRefId(field)}
		nullable := !pluginOptions.GetMandatoryNullable() && (inRealOneof(field) || isOptionalField(field)) || proto.GetFieldOptions(field).GetNullable()
		if nullable && !acceptsNull(field) {
			fieldSchema = &jsonschema.Schema{OneOf: []*jsonschema.Schema{
				{Type: "null"},
				fieldSchema,
			}}
		}

		if isRequiredField(pluginOptions, behaviors, field) {
			// If field is not a member of oneOf
			schema.Required = append(schema.Required, propName)
		}

		schema.Properties.Set(propName, fieldSchema)
	}

	// Convert Protobuf OneOfs to JSONSchema keywords. A oneof with a single
	// member (the synthetic oneof of a proto3 optional field, or one left with
	// a single visible field) would constrain nothing.
	for oneOf := range listOf(message.Oneofs()) {
		if oneOf.IsSynthetic() {
			continue
		}
		var oneOfSchemas []*jsonschema.Schema
		for field := range listOf(oneOf.Fields()) {
			if isHiddenField(pluginOptions, field) {
				continue
			}
			propName := toPropertyName(field, pluginOptions.GetPreserveProtoFieldNames())
			oneOfSchemas = append(oneOfSchemas, &jsonschema.Schema{Required: []string{propName}})
		}
		if len(oneOfSchemas) < 2 {
			continue
		}

		negativeSchema := &jsonschema.Schema{Not: &jsonschema.Schema{AnyOf: slices.Clone(oneOfSchemas)}}

		combinedSchemas := append(oneOfSchemas, negativeSchema)
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{OneOf: combinedSchemas})
	}
	return schema
}

// isRequiredField reports whether field goes into required. In order of
// precedence: the required field option, a proto2 required or editions
// LEGACY_REQUIRED field, the REQUIRED field behavior and, unless
// respect_protojson_presence is set, every field that is not optional,
// repeated, a map or in a oneof.
func isRequiredField(pluginOptions *proto.PluginOptions, behaviors fieldBehaviors, field protoreflect.FieldDescriptor) bool {
	if fo := proto.GetFieldOptions(field); fo != nil && fo.Required != nil {
		return fo.GetRequired()
	}
	if field.Cardinality() == protoreflect.Required {
		return true
	}
	if !inRealOneof(field) && behaviors.has(field, "REQUIRED") {
		return true
	}
	if pluginOptions.GetRespectProtojsonPresence() {
		// Per ProtoJSON spec, parsers must accept absent fields and use default values,
		// so no field is required. See https://protobuf.dev/programming-guides/json/
		return false
	}
	return !inRealOneof(field) && !isOptionalField(field) && !field.IsList() && !field.IsMap()
}

func buildFromWellKnownMessage(pluginOptions *proto.PluginOptions, behaviors fieldBehaviors, message protoreflect.MessageDescriptor, mo *proto.MessageOptions) *jsonschema.Schema {
	baseSchema := buildFromMessage(pluginOptions, behaviors, message, mo)

	wellKnownType := getWellKnownMessageType(message)
	if wellKnownType == WellKnownMessageTypeNone {
		panic("not well known type")
	}
	switch wellKnownType {
	case WellKnownMessageTypeK8sIntOrString:
		schema := &jsonschema.Schema{}
		schema.Title = baseSchema.Title
		schema.Description = baseSchema.Description
		schema.OneOf = []*jsonschema.Schema{
			{Type: "string"},
			{Type: "integer"},
		}
		return schema
	case WellKnownMessageTypeK8sVolume:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.VolumeSource"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "volumeSource")
		baseSchema.Properties.Delete("volumeSource")
	case WellKnownMessageTypeK8sSecretProjection:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapVolumeSource:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapProjection:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapKeySelector:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sSecretKeySelector:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sConfigMapEnvSource:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sSecretEnvSource:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.LocalObjectReference"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "localObjectReference")
		baseSchema.Properties.Delete("localObjectReference")
	case WellKnownMessageTypeK8sProbe:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.ProbeHandler"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "handler")
		baseSchema.Properties.Delete("handler")
	case WellKnownMessageTypeK8sEphemeralContainer:
		baseSchema.OneOf = []*jsonschema.Schema{{Ref: ".k8s.io.api.core.v1.EphemeralContainerCommon"}}
		baseSchema.Required = deletePropertyInRequired(baseSchema.Required, "ephemeralContainerCommon")
		baseSchema.Properties.Delete("ephemeralContainerCommon")
	}
	return baseSchema
}

type WellKnownMessageType int

const (
	WellKnownMessageTypeNone WellKnownMessageType = iota
	WellKnownMessageTypeK8sIntOrString
	WellKnownMessageTypeK8sVolume
	WellKnownMessageTypeK8sSecretProjection
	WellKnownMessageTypeK8sConfigMapVolumeSource
	WellKnownMessageTypeK8sConfigMapProjection
	WellKnownMessageTypeK8sConfigMapKeySelector
	WellKnownMessageTypeK8sSecretKeySelector
	WellKnownMessageTypeK8sConfigMapEnvSource
	WellKnownMessageTypeK8sSecretEnvSource
	WellKnownMessageTypeK8sProbe
	WellKnownMessageTypeK8sEphemeralContainer
)

func isWellKnownMessage(message protoreflect.MessageDescriptor) bool {
	return getWellKnownMessageType(message) != WellKnownMessageTypeNone
}

func getWellKnownMessageType(message protoreflect.MessageDescriptor) WellKnownMessageType {
	switch fullyQualifiedName(message) {
	case ".k8s.io.apimachinery.pkg.util.intstr.IntOrString":
		return WellKnownMessageTypeK8sIntOrString
	case ".k8s.io.api.core.v1.Volume":
		return WellKnownMessageTypeK8sVolume
	case ".k8s.io.api.core.v1.SecretProjection":
		return WellKnownMessageTypeK8sSecretProjection
	case ".k8s.io.api.core.v1.ConfigMapVolumeSource":
		return WellKnownMessageTypeK8sConfigMapVolumeSource
	case ".k8s.io.api.core.v1.ConfigMapProjection":
		return WellKnownMessageTypeK8sConfigMapProjection
	case ".k8s.io.api.core.v1.ConfigMapKeySelector":
		return WellKnownMessageTypeK8sConfigMapKeySelector
	case ".k8s.io.api.core.v1.SecretKeySelector":
		return WellKnownMessageTypeK8sSecretKeySelector
	case ".k8s.io.api.core.v1.ConfigMapEnvSource":
		return WellKnownMessageTypeK8sConfigMapEnvSource
	case ".k8s.io.api.core.v1.SecretEnvSource":
		return WellKnownMessageTypeK8sSecretEnvSource
	case ".k8s.io.api.core.v1.Probe":
		return WellKnownMessageTypeK8sProbe
	case ".k8s.io.api.core.v1.EphemeralContainer":
		return WellKnownMessageTypeK8sEphemeralContainer
	}

	return WellKnownMessageTypeNone
}

// buildFromField builds the schema of a field that is not a map.
func buildFromField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := buildFromValue(pluginOptions, field, fo)
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)

	if field.IsList() {
		return wrapSchemaInArray(schema, field, fo)
	}
	return schema
}

func buildFromMapField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)
	schema.Type = "object"
	schema.PropertyNames = mapKeySchema(field.MapKey().Kind())
	schema.AdditionalProperties = buildFromValue(pluginOptions, field.MapValue(), fo)
	return schema
}

// mapKeySchema returns the schema of the keys of a map: ProtoJSON writes
// integer and bool keys as strings.
func mapKeySchema(kind protoreflect.Kind) *jsonschema.Schema {
	switch {
	case kind == protoreflect.BoolKind:
		return &jsonschema.Schema{Enum: []any{"true", "false"}}
	case isIntKind(kind):
		return &jsonschema.Schema{Pattern: integerPattern(kind)}
	}
	return nil
}

// integerPattern matches an integer of kind written as a decimal string.
func integerPattern(kind protoreflect.Kind) string {
	if isUnsignedKind(kind) {
		return "^(0|[1-9][0-9]*)$"
	}
	return "^-?(0|[1-9][0-9]*)$"
}

// acceptsNull reports whether the schema of field already accepts null:
// ProtoJSON writes a NullValue and a null Value as null.
func acceptsNull(field protoreflect.FieldDescriptor) bool {
	if field.IsList() || field.IsMap() {
		return false
	}
	switch {
	case field.Kind() == protoreflect.EnumKind:
		return field.Enum().FullName() == "google.protobuf.NullValue"
	case isMessageKind(field.Kind()):
		return field.Message().FullName() == "google.protobuf.Value"
	}
	return false
}

// buildFromValue builds the schema of one value of field as ProtoJSON
// writes it: the field itself, an element of a repeated field or the value
// of a map.
func buildFromValue(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	kind := field.Kind()
	switch {
	case isMessageKind(kind):
		if wkt := buildFromWellKnownType(pluginOptions, field.Message(), fo); wkt != nil {
			return wkt
		}
		schema.Ref = toRefId(field.Message())
	case kind == protoreflect.EnumKind:
		if field.Enum().FullName() == "google.protobuf.NullValue" {
			schema.Type = "null"
		} else {
			schema.Ref = toRefId(field.Enum())
		}
	case isIntKind(kind):
		if pluginOptions.GetRespectProtojsonInt64() && isInt64(kind) {
			// Not format "int64": no draft defines it, and validators such
			// as Ajv refuse unknown formats.
			schema.Type = "string"
			schema.Pattern = integerPattern(kind)
		} else {
			schema.Type = "integer"
			fillSchemaByNumericKeywords(schema, fo.GetNumeric())
		}
	case isNumericKind(kind):
		schema.Type = "number"
		if fo.GetNumeric() != nil {
			// Numeric keywords bound the value, which rules out NaN and
			// the infinities.
			fillSchemaByNumericKeywords(schema, fo.GetNumeric())
			break
		}
		// ProtoJSON writes NaN and the infinities as strings.
		return &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
			schema,
			{Type: "string", Enum: []any{"NaN", "Infinity", "-Infinity"}},
		}}
	case kind == protoreflect.BoolKind:
		schema.Type = "boolean"
	case kind == protoreflect.StringKind:
		schema.Type = "string"
		fillSchemaByStringKeywords(schema, fo.GetString_())
	case kind == protoreflect.BytesKind:
		schema.Type = "string"
		// Base64 Regex Expression
		schema.Pattern = "^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
		fillSchemaByStringKeywords(schema, fo.GetString_())
	}
	return schema
}

// buildFromWellKnownType returns the schema of a google.protobuf message
// with a special ProtoJSON form, or nil for any other message.
// See https://protobuf.dev/programming-guides/json/
func buildFromWellKnownType(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	switch message.FullName() {
	case "google.protobuf.Timestamp":
		schema := &jsonschema.Schema{Type: "string", Format: "date-time"}
		fillSchemaByStringKeywords(schema, fo.GetString_())
		return schema
	case "google.protobuf.Duration":
		// Seconds with up to 9 fractional digits, like "1.5s"; not the
		// ISO 8601 duration of format "duration".
		schema := &jsonschema.Schema{Type: "string", Pattern: `^-?[0-9]+(\.[0-9]{1,9})?s$`}
		fillSchemaByStringKeywords(schema, fo.GetString_())
		return schema
	case "google.protobuf.FieldMask":
		schema := &jsonschema.Schema{Type: "string"}
		fillSchemaByStringKeywords(schema, fo.GetString_())
		return schema
	case "google.protobuf.Any", "google.protobuf.Struct":
		return &jsonschema.Schema{Type: "object"}
	case "google.protobuf.ListValue":
		return &jsonschema.Schema{Type: "array"}
	case "google.protobuf.Value":
		// Any JSON value.
		return &jsonschema.Schema{}
	case "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
		"google.protobuf.Int64Value", "google.protobuf.UInt64Value",
		"google.protobuf.Int32Value", "google.protobuf.UInt32Value",
		"google.protobuf.BoolValue", "google.protobuf.StringValue", "google.protobuf.BytesValue":
		// A wrapper is written as its value.
		if value := message.Fields().ByName("value"); value != nil {
			return buildFromValue(pluginOptions, value, fo)
		}
	}
	return nil
}

func buildFromEnum(enum protoreflect.EnumDescriptor) (*jsonschema.Schema, error) {
	eo := proto.GetEnumOptions(enum)

	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(eo)
	schema.Description = proto.GetDescriptionOrComment(enum, eo)

	for enumValue := range listOf(enum.Values()) {
		var value any
		switch eo.GetMappingType() {
		case proto.EnumOptions_MapToString:
			value = string(enumValue.Name())
		case proto.EnumOptions_MapToNumber:
			value = int32(enumValue.Number())
		case proto.EnumOptions_MapToCustom:
			evo := proto.GetEnumValueOptions(enumValue)

			customValue, err := parseScalaValueFromAny(evo.GetCustomValue())
			if err != nil {
				return nil, err
			}
			value = customValue
			if value == nil {
				value = string(enumValue.Name())
			}
		}

		// Aliases (allow_alias) share a number, and custom values may repeat;
		// draft-04/06/07 require unique enum items.
		if !slices.ContainsFunc(schema.Enum, func(v any) bool { return reflect.DeepEqual(v, value) }) {
			schema.Enum = append(schema.Enum, value)
		}
	}

	switch eo.GetMappingType() {
	case proto.EnumOptions_MapToString:
		schema.Type = "string"
	case proto.EnumOptions_MapToNumber:
		schema.Type = "integer"
	case proto.EnumOptions_MapToCustom:
		// Custom values may be of any JSON type; only narrow when all agree.
		if !slices.ContainsFunc(schema.Enum, func(v any) bool { _, ok := v.(string); return !ok }) {
			schema.Type = "string"
		}
	}
	return schema, nil
}

func isHiddenMessage(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor) bool {
	return proto.GetMessageOptions(message).GetVisibilityLevel() < pluginOptions.GetVisibilityLevel()
}

func isHiddenField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor) bool {
	return proto.GetFieldOptions(field).GetVisibilityLevel() < pluginOptions.GetVisibilityLevel()
}

func wrapSchemaInArray(schema *jsonschema.Schema, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	repeatedSchema := &jsonschema.Schema{}
	repeatedSchema.Title = schema.Title
	repeatedSchema.Description = schema.Description
	repeatedSchema.Type = "array"
	repeatedSchema.Items = schema

	fillSchemaByArrayKeywords(repeatedSchema, fo.GetArray())
	return repeatedSchema
}

func parseScalaValueFromAny(anyValue *anypb.Any) (any, error) {
	if anyValue == nil || anyValue.Value == nil {
		return nil, nil
	}

	var value any
	if err := json.Unmarshal(anyValue.Value, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func toPropertyName(field protoreflect.FieldDescriptor, preserveProto bool) string {
	if !preserveProto && field.HasJSONName() {
		return field.JSONName()
	}
	return string(field.Name())
}

// fullyQualifiedName returns the name of desc with a leading dot, as in
// ".pkg.Message.field"; it is the key of the schema of desc.
func fullyQualifiedName(desc protoreflect.Descriptor) string {
	return "." + string(desc.FullName())
}

func toRefId(desc protoreflect.Descriptor) jsonschema.RefId {
	return jsonschema.RefId(fullyQualifiedName(desc))
}

// isMessageKind reports whether kind is a message. Groups count: in
// editions a message field with DELIMITED encoding is a group, and a proto2
// group is a message in JSON too.
func isMessageKind(kind protoreflect.Kind) bool {
	return kind == protoreflect.MessageKind || kind == protoreflect.GroupKind
}

// isOptionalField reports whether field is singular, outside a oneof and
// may be absent: an optional field in proto2 or proto3, or a field with
// explicit presence in an editions file. Message fields follow the
// field_presence feature too, so an editions file migrated from proto2 or
// proto3 keeps its schema.
func isOptionalField(field protoreflect.FieldDescriptor) bool {
	if field.ParentFile().Syntax() != protoreflect.Editions {
		return field.HasOptionalKeyword()
	}
	return field.Cardinality() == protoreflect.Optional && field.ContainingOneof() == nil &&
		fieldPresence(field) == descriptorpb.FeatureSet_EXPLICIT
}

// fieldPresence resolves the field_presence feature of a field in an
// editions file: the closest of the field, its messages and its file that
// sets it, or EXPLICIT, the default of every edition.
func fieldPresence(field protoreflect.FieldDescriptor) descriptorpb.FeatureSet_FieldPresence {
	for d := protoreflect.Descriptor(field); d != nil; d = d.Parent() {
		if p := featuresOf(d).GetFieldPresence(); p != descriptorpb.FeatureSet_FIELD_PRESENCE_UNKNOWN {
			return p
		}
	}
	return descriptorpb.FeatureSet_EXPLICIT
}

func featuresOf(d protoreflect.Descriptor) *descriptorpb.FeatureSet {
	switch opts := d.Options().(type) {
	case *descriptorpb.FieldOptions:
		return opts.GetFeatures()
	case *descriptorpb.MessageOptions:
		return opts.GetFeatures()
	case *descriptorpb.FileOptions:
		return opts.GetFeatures()
	}
	return nil
}

// inRealOneof reports whether field is in a oneof written in the .proto
// file, not the synthetic oneof of a proto3 optional field.
func inRealOneof(field protoreflect.FieldDescriptor) bool {
	oneof := field.ContainingOneof()
	return oneof != nil && !oneof.IsSynthetic()
}

func isIntKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func isUnsignedKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func isNumericKind(kind protoreflect.Kind) bool {
	return isIntKind(kind) || kind == protoreflect.DoubleKind || kind == protoreflect.FloatKind
}

func isInt64(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int64Kind, protoreflect.Uint64Kind, protoreflect.Sfixed64Kind, protoreflect.Sint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

func listOf[T any](list interface {
	Len() int
	Get(int) T
}) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := range list.Len() {
			if !yield(list.Get(i)) {
				return
			}
		}
	}
}

func deletePropertyInRequired(required []string, item string) []string {
	return slices.DeleteFunc(required, func(val string) bool {
		return val == item
	})
}
