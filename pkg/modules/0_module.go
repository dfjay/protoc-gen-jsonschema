package modules

import (
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	goproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// Generate runs the plugin on a protoc request. Errors go into the response
// for protoc to report.
func Generate(req *pluginpb.CodeGeneratorRequest, logw io.Writer, debug bool) *pluginpb.CodeGeneratorResponse {
	resp := &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: new(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}
	files, err := generate(req, logger{w: logw, debug: debug})
	if err != nil {
		resp.Error = new(err.Error())
		return resp
	}
	resp.File = files
	return resp
}

func generate(req *pluginpb.CodeGeneratorRequest, log logger) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	pluginOptions, err := proto.GetPluginOptions(proto.ParseParameters(req.GetParameter()))
	if err != nil {
		return nil, fmt.Errorf("invalid plugin options: %w", err)
	}
	log.Debugf("pluginOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(pluginOptions))

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{File: withoutMessageSets(req.GetProtoFile())})
	if err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Imported files too: their types can appear in a schema.
	visitor := NewVisitor(pluginOptions)
	for _, fdp := range req.GetProtoFile() {
		file, err := files.FindFileByPath(fdp.GetName())
		if err != nil {
			return nil, err
		}
		if err := visitor.VisitFile(file); err != nil {
			return nil, fmt.Errorf("%s: %w", file.Path(), err)
		}
	}
	log.Debugf("# of IntermediateSchemas: %d", len(visitor.registry.GetKeys()))
	visitor.registry.SortSchemas()

	backend := &backend{
		log:        log,
		options:    pluginOptions,
		optimizer:  NewOptimizerImpl(),
		generator:  NewMultiDraftGenerator(pluginOptions),
		serializer: NewSerializerImpl(pluginOptions),
	}
	var out []*pluginpb.CodeGeneratorResponse_File
	for _, name := range req.GetFileToGenerate() {
		file, err := files.FindFileByPath(name)
		if err != nil {
			return nil, err
		}
		generated, err := backend.generate(file, visitor.registry)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if generated != nil {
			out = append(out, generated)
		}
	}

	if len(req.GetFileToGenerate()) > 0 && len(out) == 0 {
		log.Logf("No schema generated: a schema is written for each input file that defines its entrypoint message. " +
			"Pass --jsonschema_opt=entrypoint_message=<Message> or set option (dfjay.jsonschema.file) = {entrypoint_message: \"<Message>\"} in the file.")
	}
	return out, nil
}

type backend struct {
	log        logger
	options    *proto.PluginOptions
	optimizer  BackendOptimizer
	generator  BackendTargetGenerator
	serializer BackendSerializer
}

// generate returns the schema file of file, or nil when file has no
// entrypoint message.
func (b *backend) generate(file protoreflect.FileDescriptor, registry *jsonschema.Registry) (*pluginpb.CodeGeneratorResponse_File, error) {
	log := b.log.with(file.Path())
	fileOptions := proto.GetFileOptions(file)
	log.Debugf("FileOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(fileOptions))

	entrypointName := proto.GetEntrypointMessage(b.options, fileOptions)
	if entrypointName == "" {
		log.Debugf("No entrypoint message set; skipping")
		return nil, nil
	}
	entrypointMessage := file.Messages().ByName(protoreflect.Name(entrypointName))
	if entrypointMessage == nil {
		log.Logf("Entrypoint message %q is not defined in this file; skipping", entrypointName)
		return nil, nil
	}

	copiedRegistry := jsonschema.DeepCopyRegistry(registry)
	if err := b.optimizer.Optimize(copiedRegistry, entrypointMessage); err != nil {
		return nil, fmt.Errorf("failed to generate schema for entrypoint %s: %w", fullyQualifiedName(entrypointMessage), err)
	}
	log.Debugf("# of Schemas After Optimized : %d", len(copiedRegistry.GetKeys()))

	rootSchema := b.generator.Generate(copiedRegistry, entrypointMessage, fileOptions)
	if rootSchema == nil {
		return nil, fmt.Errorf("unsupported draft %s", b.options.GetDraft())
	}

	fileName := b.serializer.ToFileName(file)
	content, err := b.serializer.Serialize(rootSchema, file)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize %s: %w", fileName, err)
	}
	log.Debugf("GeneratedFileName: %s", fileName)
	return &pluginpb.CodeGeneratorResponse_File{Name: new(fileName), Content: new(string(content))}, nil
}

func outputFileName(protoPath, suffix string) string {
	return path.Clean(strings.TrimSuffix(protoPath, path.Ext(protoPath)) + suffix)
}

// withoutMessageSets turns MessageSets, a proto1 feature that protodesc
// rejects, into plain messages: it clears message_set_wire_format and caps
// their extension ranges at the largest valid field number. Neither matters
// for JSON.
func withoutMessageSets(files []*descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	out := make([]*descriptorpb.FileDescriptorProto, len(files))
	for i, file := range files {
		if !slices.ContainsFunc(file.GetMessageType(), hasMessageSet) {
			out[i] = file
			continue
		}
		file = goproto.Clone(file).(*descriptorpb.FileDescriptorProto)
		for _, message := range file.GetMessageType() {
			clearMessageSet(message)
		}
		out[i] = file
	}
	return out
}

func hasMessageSet(message *descriptorpb.DescriptorProto) bool {
	return message.GetOptions().GetMessageSetWireFormat() || slices.ContainsFunc(message.GetNestedType(), hasMessageSet)
}

func clearMessageSet(message *descriptorpb.DescriptorProto) {
	if message.GetOptions().GetMessageSetWireFormat() {
		message.Options.MessageSetWireFormat = nil
		for _, r := range message.GetExtensionRange() {
			r.End = new(min(r.GetEnd(), int32(protowire.MaxValidNumber)+1))
		}
	}
	for _, nested := range message.GetNestedType() {
		clearMessageSet(nested)
	}
}

// logger writes to protoc's stderr.
type logger struct {
	w      io.Writer
	debug  bool
	prefix string
}

func (l logger) with(prefix string) logger {
	l.prefix = prefix + ": "
	return l
}

func (l logger) Logf(format string, args ...any) {
	if l.w != nil {
		_, _ = fmt.Fprintf(l.w, "protoc-gen-jsonschema: %s%s\n", l.prefix, fmt.Sprintf(format, args...))
	}
}

func (l logger) Debugf(format string, args ...any) {
	if l.debug {
		l.Logf(format, args...)
	}
}
