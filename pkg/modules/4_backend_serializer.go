package modules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"sigs.k8s.io/yaml"
)

type BackendSerializer interface {
	Serialize(schema any, file protoreflect.FileDescriptor) ([]byte, error)
	ToFileName(file protoreflect.FileDescriptor) string
}

var _ BackendSerializer = (*SerializerImpl)(nil)

type SerializerImpl struct {
	pluginOptions *proto.PluginOptions
}

func NewSerializerImpl(pluginOptions *proto.PluginOptions) *SerializerImpl {
	return &SerializerImpl{pluginOptions: pluginOptions}
}

func (s *SerializerImpl) Serialize(schema any, file protoreflect.FileDescriptor) ([]byte, error) {
	outputFileSuffix := s.ToFileName(file)

	if strings.HasSuffix(outputFileSuffix, ".json") {
		if s.pluginOptions.GetPrettyJsonOutput() {
			return json.MarshalIndent(schema, "", "  ")
		} else {
			return json.Marshal(schema)
		}
	} else if strings.HasSuffix(outputFileSuffix, ".yaml") || strings.HasSuffix(outputFileSuffix, ".yml") {
		return yaml.Marshal(schema)
	} else {
		return nil, fmt.Errorf("unsupported output file suffix: `%s`, suffix should be endsWith `.json`, `.yaml`, `.yml`", outputFileSuffix)
	}
}

func (s *SerializerImpl) ToFileName(file protoreflect.FileDescriptor) string {
	return outputFileName(file.Path(), s.pluginOptions.OutputFileSuffix)
}
