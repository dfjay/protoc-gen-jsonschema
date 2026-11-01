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

func buildFromMessage(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor, mo *proto.MessageOptions) *jsonschema.Schema {
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
		if !pluginOptions.GetMandatoryNullable() && (inRealOneof(field) || isOptionalField(field)) || proto.GetFieldOptions(field).GetNullable() {
			fieldSchema = &jsonschema.Schema{OneOf: []*jsonschema.Schema{
				{Type: "null"},
				fieldSchema,
			}}
		}

		if isRequiredField(pluginOptions, field) {
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

func isRequiredField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor) bool {
	// Explicit per-field required option always takes precedence
	if fo := proto.GetFieldOptions(field); fo != nil && fo.Required != nil {
		return fo.GetRequired()
	}

	if pluginOptions.GetRespectProtojsonPresence() {
		// Per ProtoJSON spec, parsers must accept absent fields and use default values,
		// so no field is required. See https://protobuf.dev/programming-guides/json/
		return false
	} else {
		return !inRealOneof(field) && !isOptionalField(field) && !field.IsList() && !field.IsMap()
	}
}

func buildFromWellKnownMessage(pluginOptions *proto.PluginOptions, message protoreflect.MessageDescriptor, mo *proto.MessageOptions) *jsonschema.Schema {
	baseSchema := buildFromMessage(pluginOptions, message, mo)

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

func buildFromMessageField(field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)

	schema.Ref = toRefId(field.Message())

	if field.IsList() {
		return wrapSchemaInArray(schema, field, fo)
	} else {
		return schema
	}
}

func buildFromMapField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)
	schema.Type = "object"

	valueSchema := &jsonschema.Schema{}
	value := field.MapValue()
	kind := value.Kind()
	if isIntKind(kind) {
		if pluginOptions.GetRespectProtojsonInt64() && isInt64(kind) {
			valueSchema.Type = "string"
			valueSchema.Format = "int64"
		} else {
			valueSchema.Type = "integer"
			fillSchemaByNumericKeywords(valueSchema, fo.GetNumeric())
		}
	} else if isNumericKind(kind) {
		valueSchema.Type = "number"
		fillSchemaByNumericKeywords(valueSchema, fo.GetNumeric())
	} else if isMessageKind(kind) {
		valueSchema.Ref = toRefId(value.Message())
	} else if kind == protoreflect.BoolKind {
		valueSchema.Type = "boolean"
	} else if kind == protoreflect.EnumKind {
		valueSchema.Ref = toRefId(value.Enum())
	} else if kind == protoreflect.StringKind {
		valueSchema.Type = "string"
	} else if kind == protoreflect.BytesKind {
		valueSchema.Type = "string"
		// Base64 Regex Expression
		valueSchema.Pattern = "^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
	}
	schema.AdditionalProperties = valueSchema
	return schema
}

func buildFromScalaField(pluginOptions *proto.PluginOptions, field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)

	kind := field.Kind()
	if isIntKind(kind) {
		if pluginOptions.GetRespectProtojsonInt64() && isInt64(kind) {
			schema.Type = "string"
			schema.Format = "int64"
		} else {
			schema.Type = "integer"
			fillSchemaByNumericKeywords(schema, fo.GetNumeric())
		}
	} else if kind == protoreflect.DoubleKind || kind == protoreflect.FloatKind {
		schema.Type = "number"
		fillSchemaByNumericKeywords(schema, fo.GetNumeric())
	} else if kind == protoreflect.BoolKind {
		schema.Type = "boolean"
	} else if kind == protoreflect.EnumKind {
		schema.Ref = toRefId(field.Enum())
	} else if kind == protoreflect.StringKind {
		schema.Type = "string"
		fillSchemaByStringKeywords(schema, fo.GetString_())
	} else if kind == protoreflect.BytesKind {
		schema.Type = "string"
		// Base64 Regex Expression
		schema.Pattern = "^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
		fillSchemaByStringKeywords(schema, fo.GetString_())
	}

	if field.IsList() {
		return wrapSchemaInArray(schema, field, fo)
	} else {
		return schema
	}
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

func buildFromWellKnownField(field protoreflect.FieldDescriptor, fo *proto.FieldOptions) *jsonschema.Schema {
	schema := &jsonschema.Schema{}
	schema.Title = proto.GetTitleOrEmpty(fo)
	schema.Description = proto.GetDescriptionOrComment(field, fo)

	wellKnownType := getWellKnownFieldType(field)
	if wellKnownType == WellKnownTypeNone {
		panic("not well known type")
	}

	switch wellKnownType {
	case WellKnownTypeTimestamp:
		schema.Type = "string"
		schema.Format = "date-time"
		fillSchemaByStringKeywords(schema, fo.GetString_())
	case WellKnownTypeDuration:
		schema.Type = "string"
		schema.Format = "duration"
		fillSchemaByStringKeywords(schema, fo.GetString_())
	case WellKnownTypeAny:
		schema.Type = "object"
	case WellKnownTypeNullValue:
		schema.Type = "null"
	}

	if field.IsList() {
		return wrapSchemaInArray(schema, field, fo)
	}
	return schema
}

func isWellKnownField(field protoreflect.FieldDescriptor) bool {
	return getWellKnownFieldType(field) != WellKnownTypeNone
}

type WellKnownFieldType int

const (
	WellKnownTypeNone WellKnownFieldType = iota
	WellKnownTypeTimestamp
	WellKnownTypeDuration
	WellKnownTypeAny
	WellKnownTypeNullValue
)

func getWellKnownFieldType(field protoreflect.FieldDescriptor) WellKnownFieldType {
	if field.IsMap() || !isMessageKind(field.Kind()) {
		return WellKnownTypeNone
	}

	switch fullyQualifiedName(field.Message()) {
	case ".google.protobuf.Timestamp":
		return WellKnownTypeTimestamp
	case ".google.protobuf.Duration":
		return WellKnownTypeDuration
	case ".google.protobuf.Any":
		return WellKnownTypeAny
	case ".google.protobuf.NullValue":
		return WellKnownTypeNullValue
	default:
		return WellKnownTypeNone
	}
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

func isScalarType(field protoreflect.FieldDescriptor) bool {
	kind := field.Kind()
	if isNumericKind(kind) {
		return true
	}
	if kind == protoreflect.BoolKind {
		return true
	}
	if kind == protoreflect.EnumKind {
		return true
	}
	if kind == protoreflect.StringKind || kind == protoreflect.BytesKind {
		return true
	}
	return false
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
