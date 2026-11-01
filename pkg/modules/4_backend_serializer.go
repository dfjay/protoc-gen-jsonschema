package modules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"sigs.k8s.io/yaml"
)

type BackendSerializer interface {
	// Serialize picks JSON or YAML by the extension of fileName.
	Serialize(schema any, fileName string) ([]byte, error)
}

var _ BackendSerializer = (*SerializerImpl)(nil)

type SerializerImpl struct {
	pluginOptions *proto.PluginOptions
}

func NewSerializerImpl(pluginOptions *proto.PluginOptions) *SerializerImpl {
	return &SerializerImpl{pluginOptions: pluginOptions}
}

func (s *SerializerImpl) Serialize(schema any, fileName string) ([]byte, error) {
	switch {
	case strings.HasSuffix(fileName, ".json"):
		if s.pluginOptions.GetPrettyJsonOutput() {
			return json.MarshalIndent(schema, "", "  ")
		}
		return json.Marshal(schema)
	case strings.HasSuffix(fileName, ".yaml"), strings.HasSuffix(fileName, ".yml"):
		return yaml.Marshal(schema)
	}
	return nil, fmt.Errorf("unsupported output file suffix: `%s`, suffix should be endsWith `.json`, `.yaml`, `.yml`", s.pluginOptions.GetOutputFileSuffix())
}
