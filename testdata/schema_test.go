package testdata

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/modules"
	options "github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/pluginpb"
	"pgregory.net/rapid"
	"sigs.k8s.io/yaml"
)

// compileSchema checks the schema against its draft's meta-schema and resolves every $ref.
func compileSchema(name, content string) (*jsonschema.Schema, error) {
	data := []byte(content)
	if ext := filepath.Ext(name); ext == ".yaml" || ext == ".yml" {
		var err error
		if data, err = yaml.YAMLToJSON(data); err != nil {
			return nil, err
		}
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	url := "file:///" + name
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

// TestRoundTrip checks that the schema accepts what protojson writes for random messages.
func TestRoundTrip(t *testing.T) {
	optionsImportPath, err := filepath.Abs(optionsDir)
	if err != nil {
		t.Fatal(err)
	}
	forEachCase(t, func(t *testing.T, tc *Testcase, dirPath string) {
		if tc.ExpectFail || tc.ExpectResultIsNull {
			t.Skip("no schema")
		}
		if tc.SkipRoundTrip != "" {
			t.Skip(tc.SkipRoundTrip)
		}
		req := buildRequest(t, tc, dirPath, optionsImportPath)
		resp := modules.Generate(req, nil, false)
		if resp.Error != nil {
			t.Fatal(resp.GetError())
		}
		g := newGenerator(t, req)
		for _, file := range resp.File {
			t.Run(file.GetName(), func(t *testing.T) {
				schema, err := compileSchema(file.GetName(), file.GetContent())
				if err != nil {
					t.Fatal(err)
				}
				message := g.entrypoint(t, file.GetContent(), file.GetName())
				rapid.Check(t, func(rt *rapid.T) {
					g.skippedConstrained = false
					m := g.message(rt, message, 0)
					opts := protojson.MarshalOptions{
						Resolver:      g.types,
						UseProtoNames: g.opts.GetPreserveProtoFieldNames(),
						// Without presence, fields holding defaults are required too. Defaults of
						// skipped annotated fields may break their annotations.
						EmitDefaultValues: !g.opts.GetRespectProtojsonPresence() ||
							!g.skippedConstrained && rapid.Bool().Draw(rt, "emitDefaultValues"),
					}
					if g.opts.GetProtojsonMode() == options.ProtojsonMode_Lenient {
						opts.UseEnumNumbers = rapid.Bool().Draw(rt, "useEnumNumbers")
					}
					out, err := opts.Marshal(m)
					if err != nil {
						rt.Fatalf("marshal: %v", err)
					}
					doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
					if err != nil {
						rt.Fatal(err)
					}
					if err := schema.Validate(doc); err != nil {
						rt.Fatalf("%s\nprotojson wrote:\n%s", err, out)
					}
				})
			})
		}
	})
}

type generator struct {
	opts      *options.PluginOptions
	files     *protoregistry.Files
	types     *dynamicpb.Types
	behaviors protoreflect.ExtensionDescriptor
	anyTypes  []protoreflect.MessageDescriptor

	skippedConstrained bool
}

func newGenerator(t *testing.T, req *pluginpb.CodeGeneratorRequest) *generator {
	t.Helper()
	opts, err := options.GetPluginOptions(options.ParseParameters(req.GetParameter()))
	if err != nil {
		t.Fatal(err)
	}
	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{File: req.GetProtoFile()})
	if err != nil {
		t.Fatal(err)
	}
	g := &generator{opts: opts, files: files, types: dynamicpb.NewTypes(files)}
	if d, err := files.FindDescriptorByName("google.api.field_behavior"); err == nil {
		g.behaviors, _ = d.(protoreflect.ExtensionDescriptor)
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		g.anyTypes = appendMessages(g.anyTypes, fd.Messages())
		return true
	})
	return g
}

func appendMessages(out []protoreflect.MessageDescriptor, messages protoreflect.MessageDescriptors) []protoreflect.MessageDescriptor {
	for i := range messages.Len() {
		md := messages.Get(i)
		if !md.IsMapEntry() {
			out = append(out, md)
		}
		out = appendMessages(out, md.Messages())
	}
	return out
}

func (g *generator) entrypoint(t *testing.T, content, name string) protoreflect.MessageDescriptor {
	t.Helper()
	data := []byte(content)
	if ext := filepath.Ext(name); ext == ".yaml" || ext == ".yml" {
		var err error
		if data, err = yaml.YAMLToJSON(data); err != nil {
			t.Fatal(err)
		}
	}
	var root struct {
		Ref string `json:"$ref"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	_, fullName, ok := strings.Cut(root.Ref, "/.")
	if !ok {
		t.Fatalf("root $ref %q does not name a message", root.Ref)
	}
	d, err := g.files.FindDescriptorByName(protoreflect.FullName(fullName))
	if err != nil {
		t.Fatal(err)
	}
	return d.(protoreflect.MessageDescriptor)
}

const maxDepth = 3

func (g *generator) message(t *rapid.T, md protoreflect.MessageDescriptor, depth int) *dynamicpb.Message {
	m := dynamicpb.NewMessage(md)
	if g.wellKnown(t, m, depth) {
		return m
	}
	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		if g.hidden(fd) {
			continue
		}
		required := g.required(fd)
		if g.constrained(fd) {
			if required {
				t.Fatalf("%s is required and annotated; set skipRoundTrip", fd.FullName())
			}
			g.skippedConstrained = true
			continue
		}
		if !required && (depth >= maxDepth || !rapid.Bool().Draw(t, string(fd.Name()))) {
			continue
		}
		minLen := 0
		if required {
			minLen = 1 // protojson omits empty collections.
		}
		switch {
		case fd.IsMap():
			mp := m.Mutable(fd).Map()
			for range rapid.IntRange(minLen, 2).Draw(t, "len") {
				mp.Set(g.scalar(t, fd.MapKey(), false).MapKey(), g.value(t, fd.MapValue(), depth))
			}
		case fd.IsList():
			list := m.Mutable(fd).List()
			for range rapid.IntRange(minLen, 2).Draw(t, "len") {
				list.Append(g.value(t, fd, depth))
			}
		default:
			m.Set(fd, g.value(t, fd, depth))
			// protojson omits implicit-presence zero values.
			for required && !m.Has(fd) {
				m.Set(fd, g.value(t, fd, depth))
			}
		}
	}
	return m
}

func (g *generator) value(t *rapid.T, fd protoreflect.FieldDescriptor, depth int) protoreflect.Value {
	if fd.Message() != nil {
		return protoreflect.ValueOfMessage(g.message(t, fd.Message(), depth+1))
	}
	return g.scalar(t, fd, true)
}

func (g *generator) scalar(t *rapid.T, fd protoreflect.FieldDescriptor, specialFloats bool) protoreflect.Value {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(rapid.Bool().Draw(t, "bool"))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(rapid.Int32().Draw(t, "int32"))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(rapid.Uint32().Draw(t, "uint32"))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(rapid.Int64().Draw(t, "int64"))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(rapid.Uint64().Draw(t, "uint64"))
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(g.float(t, specialFloats)))
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(g.float(t, specialFloats))
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(rapid.String().Draw(t, "string"))
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes(rapid.SliceOf(rapid.Byte()).Draw(t, "bytes"))
	case protoreflect.EnumKind:
		values := fd.Enum().Values()
		return protoreflect.ValueOfEnum(values.Get(rapid.IntRange(0, values.Len()-1).Draw(t, "enum")).Number())
	}
	panic(fmt.Sprintf("unexpected kind %v", fd.Kind()))
}

func (g *generator) float(t *rapid.T, special bool) float64 {
	if special && rapid.IntRange(0, 9).Draw(t, "special") == 0 {
		return rapid.SampledFrom([]float64{math.NaN(), math.Inf(1), math.Inf(-1)}).Draw(t, "nonfinite")
	}
	return rapid.Float64().Draw(t, "float")
}

// wellKnown fills well-known types with values protojson can write.
func (g *generator) wellKnown(t *rapid.T, m *dynamicpb.Message, depth int) bool {
	fields := m.Descriptor().Fields()
	switch m.Descriptor().FullName() {
	case "google.protobuf.Timestamp":
		ts := time.Unix(rapid.Int64Range(-62135596800, 253402300799).Draw(t, "seconds"), int64(rapid.Int32Range(0, 999999999).Draw(t, "nanos")))
		m.Set(fields.ByName("seconds"), protoreflect.ValueOfInt64(ts.Unix()))
		m.Set(fields.ByName("nanos"), protoreflect.ValueOfInt32(int32(ts.Nanosecond())))
	case "google.protobuf.Duration":
		seconds := rapid.Int64Range(-315576000000, 315576000000).Draw(t, "seconds")
		nanos := rapid.Int32Range(0, 999999999).Draw(t, "nanos")
		if seconds < 0 || seconds == 0 && rapid.Bool().Draw(t, "negative") {
			nanos = -nanos
		}
		m.Set(fields.ByName("seconds"), protoreflect.ValueOfInt64(seconds))
		m.Set(fields.ByName("nanos"), protoreflect.ValueOfInt32(nanos))
	case "google.protobuf.FieldMask":
		paths := m.Mutable(fields.ByName("paths")).List()
		path := rapid.StringMatching(`[a-z]+(_[a-z]+)*(\.[a-z]+(_[a-z]+)*)*`)
		for range rapid.IntRange(0, 2).Draw(t, "len") {
			paths.Append(protoreflect.ValueOfString(path.Draw(t, "path")))
		}
	case "google.protobuf.Any":
		if len(g.anyTypes) == 0 || depth >= maxDepth {
			return true
		}
		inner := g.message(t, rapid.SampledFrom(g.anyTypes).Draw(t, "anyType"), depth+1)
		value, err := proto.MarshalOptions{Deterministic: true}.Marshal(inner)
		if err != nil {
			t.Fatal(err)
		}
		m.Set(fields.ByName("type_url"), protoreflect.ValueOfString("type.googleapis.com/"+string(inner.Descriptor().FullName())))
		m.Set(fields.ByName("value"), protoreflect.ValueOfBytes(value))
	case "google.protobuf.Value":
		// protojson rejects a non-finite number_value.
		kinds := []protoreflect.Name{"null_value", "number_value", "string_value", "bool_value", "struct_value", "list_value"}
		if depth >= maxDepth {
			kinds = kinds[:4]
		}
		fd := fields.ByName(rapid.SampledFrom(kinds).Draw(t, "kind"))
		switch fd.Kind() {
		case protoreflect.DoubleKind:
			m.Set(fd, protoreflect.ValueOfFloat64(g.float(t, false)))
		case protoreflect.EnumKind:
			m.Set(fd, protoreflect.ValueOfEnum(0))
		default:
			m.Set(fd, g.value(t, fd, depth))
		}
	default:
		return false
	}
	return true
}

func (g *generator) hidden(fd protoreflect.FieldDescriptor) bool {
	return options.GetFieldOptions(fd).GetVisibilityLevel() < g.opts.GetVisibilityLevel() ||
		(fd.Message() != nil && options.GetMessageOptions(fd.Message()).GetVisibilityLevel() < g.opts.GetVisibilityLevel())
}

// required mirrors isRequiredField in pkg/modules.
func (g *generator) required(fd protoreflect.FieldDescriptor) bool {
	if fo := options.GetFieldOptions(fd); fo != nil && fo.Required != nil {
		return fo.GetRequired()
	}
	if fd.Cardinality() == protoreflect.Required {
		return true
	}
	inOneof := fd.ContainingOneof() != nil && !fd.ContainingOneof().IsSynthetic()
	if !inOneof && g.opts.GetRespectFieldBehavior() && g.hasBehavior(fd, "REQUIRED") {
		return true
	}
	if g.opts.GetRespectProtojsonPresence() {
		return false
	}
	// Mirrors isOptionalField.
	optional := fd.HasOptionalKeyword()
	if fd.ParentFile().Syntax() == protoreflect.Editions {
		optional = fd.HasPresence()
	}
	return !inOneof && !optional && !fd.IsList() && !fd.IsMap()
}

func (g *generator) constrained(fd protoreflect.FieldDescriptor) bool {
	fo := options.GetFieldOptions(fd)
	if fo.GetNumeric() != nil || fo.GetString_() != nil || fo.GetArray() != nil {
		return true
	}
	if fd.Enum() != nil && options.GetEnumOptions(fd.Enum()).GetMappingType() != options.EnumOptions_MapToString {
		return true
	}
	if md := fd.Message(); md != nil {
		obj := options.GetMessageOptions(md).GetObject()
		return obj.GetMinProperties() > 0 || obj.GetMaxProperties() > 0
	}
	return false
}

func (g *generator) hasBehavior(fd protoreflect.FieldDescriptor, name protoreflect.Name) bool {
	if g.behaviors == nil {
		return false
	}
	want := g.behaviors.Enum().Values().ByName(name).Number()
	b := fd.Options().ProtoReflect().GetUnknown()
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		b = b[n:]
		m := protowire.ConsumeFieldValue(num, typ, b)
		value := b[:m]
		b = b[m:]
		if num != g.behaviors.Number() {
			continue
		}
		if typ == protowire.BytesType {
			value, _ = protowire.ConsumeBytes(value)
		}
		for len(value) > 0 {
			v, n := protowire.ConsumeVarint(value)
			value = value[n:]
			if protoreflect.EnumNumber(v) == want {
				return true
			}
		}
	}
	return false
}
