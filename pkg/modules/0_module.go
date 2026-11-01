package modules

import (
	"fmt"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	pgs "github.com/lyft/protoc-gen-star/v2"
	"google.golang.org/protobuf/encoding/protojson"
)

type Module struct {
	*pgs.ModuleBase
	pluginOptions *proto.PluginOptions
}

func NewModule() *Module {
	return &Module{ModuleBase: &pgs.ModuleBase{}}
}

func (m *Module) Name() string {
	return "JsonSchemaModule"
}

func (m *Module) InitContext(c pgs.BuildContext) {
	m.ModuleBase.InitContext(c)
	var err error
	m.pluginOptions, err = proto.GetPluginOptions(c.Parameters())
	m.CheckErr(err, "invalid plugin options")

	m.Debugf("pluginOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(m.pluginOptions))
}

func (m *Module) Execute(targets map[string]pgs.File, packages map[string]pgs.Package) []pgs.Artifact {
	// Phase: Frontend IntermediateSchemaGenerate
	visitor := NewVisitor(m, m.pluginOptions)
	for _, pkg := range packages {
		m.CheckErr(pgs.Walk(visitor, pkg), fmt.Sprintf("failed to walk package %s", pkg.ProtoName().String()))
	}
	m.Debugf("# of IntermediateSchemas: %d", len(visitor.registry.GetKeys()))

	// Phase: Backend TargetSchemaGenerate
	var optimizer BackendOptimizer = NewOptimizerImpl(m.ModuleBase, m.pluginOptions)
	var generator BackendTargetGenerator = NewMultiDraftGenerator(m.ModuleBase, m.pluginOptions)
	var serializer BackendSerializer = NewSerializerImpl(m.pluginOptions)
	m.Push("BackendPhase")
	visitor.registry.SortSchemas()

	for _, file := range targets {
		artifact := m.BackendPhase(file, visitor.registry, optimizer, generator, serializer)
		if artifact != nil {
			m.AddArtifact(artifact)
		}
	}

	return m.Artifacts()
}

func (m *Module) BackendPhase(file pgs.File, registry *jsonschema.Registry, optimizer BackendOptimizer, generator BackendTargetGenerator, serializer BackendSerializer) pgs.Artifact {
	defer m.Push(file.Name().String()).Pop()
	m.Debugf("FileOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(proto.GetFileOptions(file)))

	entrypointMessage := getEntrypointFromFile(file, m.pluginOptions)
	if entrypointMessage == nil {
		m.Logf("Cannot find matched entrypointMessage, Please check FileOptions")
		return nil
	}

	copiedRegistry := jsonschema.DeepCopyRegistry(registry)
	if err := optimizer.Optimize(copiedRegistry, entrypointMessage); err != nil {
		m.Failf("Failed to generate schema for entrypoint %s: %v", entrypointMessage.FullyQualifiedName(), err)
		return nil
	}
	m.Debugf("# of Schemas After Optimized : %d", len(copiedRegistry.GetKeys()))

	fileOptions := proto.GetFileOptions(file)
	rootSchema := generator.Generate(copiedRegistry, entrypointMessage, fileOptions)
	if rootSchema == nil {
		m.Logf("Cannot generate rootSchema, Please check FileOptions or PluginOptions")
		return nil
	}

	content, err := serializer.Serialize(rootSchema, file)
	m.CheckErr(err, fmt.Sprintf("Failed to serialize file %s", file.Name().String()))
	fileName := serializer.ToFileName(file)
	m.Debugf("GeneratedFileName: %s", fileName)

	return pgs.GeneratorFile{Name: fileName, Contents: string(content)}
}

func getEntrypointFromFile(file pgs.File, pluginOptions *proto.PluginOptions) pgs.Message {
	entryPointMessage := proto.GetEntrypointMessage(pluginOptions, proto.GetFileOptions(file))
	if entryPointMessage == "" {
		return nil
	}

	for _, message := range file.Messages() {
		if message.Name().String() == entryPointMessage {
			return message
		}
	}
	return nil
}
