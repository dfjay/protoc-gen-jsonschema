package modules

import (
	"fmt"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	pgs "github.com/lyft/protoc-gen-star/v2"
)

// FrontendVisitor generate intermediate jsonschema from protobuf
type FrontendVisitor struct {
	pgs.Visitor

	debugger pgs.DebuggerCommon

	registry      *jsonschema.Registry
	pluginOptions *proto.PluginOptions
}

var _ pgs.Visitor = (*FrontendVisitor)(nil)

func NewVisitor(debugger pgs.DebuggerCommon, pluginOptions *proto.PluginOptions) *FrontendVisitor {
	v := &FrontendVisitor{
		debugger:      debugger,
		registry:      jsonschema.NewRegistry(),
		pluginOptions: pluginOptions,
	}
	v.Visitor = pgs.PassThroughVisitor(v)
	return v
}

func (v *FrontendVisitor) VisitMessage(message pgs.Message) (pgs.Visitor, error) {
	if isHiddenMessage(v.pluginOptions, message) {
		return nil, nil
	}
	mo := proto.GetMessageOptions(message)

	var schema *jsonschema.Schema
	// if message is well-known type
	if isWellKnownMessage(message) {
		schema = buildFromWellKnownMessage(v.pluginOptions, message, mo)
	} else {
		schema = buildFromMessage(v.pluginOptions, message, mo)
	}
	v.registry.AddSchema(message.FullyQualifiedName(), schema)
	return v, nil
}

func (v *FrontendVisitor) VisitField(field pgs.Field) (pgs.Visitor, error) {
	if isHiddenField(v.pluginOptions, field) {
		return nil, nil
	}
	fo := proto.GetFieldOptions(field)
	if numeric := fo.GetNumeric(); numeric != nil && numeric.MultipleOf != nil && numeric.GetMultipleOf() <= 0 {
		return nil, fmt.Errorf("%s: multiple_of must be greater than 0, got %d", field.FullyQualifiedName(), numeric.GetMultipleOf())
	}

	// if field is well-known type
	if isWellKnownField(field) {
		schema := buildFromWellKnownField(field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	}

	// if field is message or map type
	fieldType := field.Type()
	if fieldType.IsMap() {
		schema := buildFromMapField(v.pluginOptions, field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	} else if fieldType.ProtoType() == pgs.MessageT {
		schema := buildFromMessageField(field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	}

	// if field is scala type
	// scala = boolean, string, number
	if isScalarType(field) {
		schema := buildFromScalaField(v.pluginOptions, field, fo)
		v.registry.AddSchema(field.FullyQualifiedName(), schema)
		return v, nil
	}

	panic("not supported field type")
}

func (v *FrontendVisitor) VisitEnum(enum pgs.Enum) (pgs.Visitor, error) {
	schema, err := buildFromEnum(enum)
	if err != nil {
		return nil, err
	}
	v.registry.AddSchema(enum.FullyQualifiedName(), schema)
	return v, nil
}
