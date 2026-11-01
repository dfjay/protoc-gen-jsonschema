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
		SupportedFeatures: new(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL |
			pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)),
		MinimumEdition: new(int32(descriptorpb.Edition_EDITION_PROTO2)),
		MaximumEdition: new(int32(descriptorpb.Edition_EDITION_2024)),
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
	visitor := NewVisitor(pluginOptions, files)
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
		out = append(out, generated...)
	}

	if len(req.GetFileToGenerate()) > 0 && len(out) == 0 {
		log.Logf("No schema generated: a schema is written for each entrypoint message. " +
			"Pass --jsonschema_opt=entrypoint_message=<Message>, set option (dfjay.jsonschema.file) = {entrypoint_message: \"<Message>\"} in the file, " +
			"or set option (dfjay.jsonschema.message) = {entrypoint: true} in the messages.")
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

type entrypoint struct {
	message  protoreflect.MessageDescriptor
	fileName string
	// Set only for entrypoint_message, which stands for the whole file.
	fileOptions *proto.FileOptions
}

func (b *backend) generate(file protoreflect.FileDescriptor, registry *jsonschema.Registry) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	log := b.log.with(file.Path())
	var out []*pluginpb.CodeGeneratorResponse_File
	for _, ep := range b.entrypoints(file, registry, log) {
		generated, err := b.generateEntrypoint(ep, registry)
		if err != nil {
			return nil, err
		}
		log.Debugf("GeneratedFileName: %s", ep.fileName)
		out = append(out, generated)
	}
	return out, nil
}

// entrypoints returns the entrypoint_message of file and its messages marked
// entrypoint. Names don't depend on how many there are, so adding one
// renames no file.
func (b *backend) entrypoints(file protoreflect.FileDescriptor, registry *jsonschema.Registry, log logger) []entrypoint {
	var eps []entrypoint
	fileOptions := proto.GetFileOptions(file)
	log.Debugf("FileOptions: %v", protojson.MarshalOptions{EmitUnpopulated: true}.Format(fileOptions))
	if name := proto.GetEntrypointMessage(b.options, fileOptions); name != "" {
		if message := file.Messages().ByName(protoreflect.Name(name)); message != nil {
			eps = append(eps, entrypoint{
				message:     message,
				fileName:    outputFileName(file.Path(), b.options.GetOutputFileSuffix()),
				fileOptions: fileOptions,
			})
		} else {
			log.Logf("Entrypoint message %q is not defined in this file; skipping", name)
		}
	}

	var walk func(messages protoreflect.MessageDescriptors)
	walk = func(messages protoreflect.MessageDescriptors) {
		for message := range listOf(messages) {
			if proto.GetMessageOptions(message).GetEntrypoint() {
				// Hidden by visibility_level.
				if registry.HasSchema(fullyQualifiedName(message)) {
					relative := strings.TrimPrefix(string(message.FullName()), string(file.Package())+".")
					eps = append(eps, entrypoint{
						message:  message,
						fileName: outputFileName(file.Path(), "."+relative+b.options.GetOutputFileSuffix()),
					})
				} else {
					log.Debugf("Entrypoint %s is hidden by visibility_level; skipping", fullyQualifiedName(message))
				}
			}
			walk(message.Messages())
		}
	}
	walk(file.Messages())
	return eps
}

func (b *backend) generateEntrypoint(ep entrypoint, registry *jsonschema.Registry) (*pluginpb.CodeGeneratorResponse_File, error) {
	optimized, err := b.optimizer.Optimize(registry, ep.message)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema for entrypoint %s: %w", fullyQualifiedName(ep.message), err)
	}

	rootSchema := b.generator.Generate(optimized, ep.message, ep.fileOptions)
	if rootSchema == nil {
		return nil, fmt.Errorf("unsupported draft %s", b.options.GetDraft())
	}

	content, err := b.serializer.Serialize(rootSchema, ep.fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize %s: %w", ep.fileName, err)
	}
	return &pluginpb.CodeGeneratorResponse_File{Name: new(ep.fileName), Content: new(string(content))}, nil
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
