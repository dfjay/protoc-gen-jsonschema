package testdata

import (
	"bytes"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/modules"
	options "github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
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
					if err := schema.Validate(doc); err != nil && !g.onlyMissing(err.(*jsonschema.ValidationError), message) {
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

	bufField, bufOneof, bufMessage protoreflect.ExtensionDescriptor

	skippedConstrained bool
}

// onlyMissing reports whether err only says that constrained fields, which
// the generator leaves unset, are missing in an instance of root.
func (g *generator) onlyMissing(err *jsonschema.ValidationError, root protoreflect.MessageDescriptor) bool {
	only := func(cause *jsonschema.ValidationError) bool { return g.onlyMissing(cause, root) }
	switch k := err.ErrorKind.(type) {
	case *kind.Required:
		md := g.messageAt(root, err.InstanceLocation)
		return md != nil && !slices.ContainsFunc(k.Missing, func(property string) bool {
			fd := md.Fields().ByName(protoreflect.Name(g.fieldNames(md)[property]))
			return fd == nil || !g.constrained(fd)
		})
	case *kind.OneOf, *kind.AnyOf:
		return slices.ContainsFunc(err.Causes, only)
	}
	return len(err.Causes) > 0 && !slices.ContainsFunc(err.Causes, func(cause *jsonschema.ValidationError) bool { return !only(cause) })
}

// messageAt returns the message at location in an instance of md, or nil.
func (g *generator) messageAt(md protoreflect.MessageDescriptor, location []string) protoreflect.MessageDescriptor {
	for i := 0; i < len(location); i++ {
		fd := md.Fields().ByName(protoreflect.Name(g.fieldNames(md)[location[i]]))
		if fd == nil {
			return nil
		}
		if fd.IsList() || fd.IsMap() {
			// The index or key of the element.
			if i++; i >= len(location) {
				return nil
			}
		}
		md = fd.Message()
		if fd.IsMap() {
			md = fd.MapValue().Message()
		}
		if md == nil {
			return nil
		}
	}
	return md
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
	if g.opts.GetRespectBufValidate() {
		find := func(name protoreflect.FullName) protoreflect.ExtensionDescriptor {
			d, _ := files.FindDescriptorByName(name)
			xd, _ := d.(protoreflect.ExtensionDescriptor)
			return xd
		}
		g.bufField, g.bufOneof, g.bufMessage = find("buf.validate.field"), find("buf.validate.oneof"), find("buf.validate.message")
	}
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
		Ref   string `json:"$ref"`
		AllOf []struct {
			Ref string `json:"$ref"`
		} `json:"allOf"`
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	// Drafts up to 07 put a $ref with siblings in allOf.
	if root.Ref == "" && len(root.AllOf) > 0 {
		root.Ref = root.AllOf[0].Ref
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
		g.set(t, m, fd, depth, required)
	}
	g.exclusive(t, m, depth)
	return m
}

// set sets fd to a random value; a required one is written by protojson.
func (g *generator) set(t *rapid.T, m *dynamicpb.Message, fd protoreflect.FieldDescriptor, depth int, required bool) {
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
		// protojson omits implicit-presence zero values. An enum with only
		// the zero value cannot do better.
		for range 100 {
			if !required || m.Has(fd) {
				break
			}
			m.Set(fd, g.value(t, fd, depth))
		}
	}
}

// exclusive satisfies the protovalidate oneof rules of m.
func (g *generator) exclusive(t *rapid.T, m *dynamicpb.Message, depth int) {
	md := m.Descriptor()
	pick := func(fields []protoreflect.FieldDescriptor) {
		for _, fd := range fields {
			if !g.hidden(fd) && !g.constrained(fd) {
				g.set(t, m, fd, depth, true)
				return
			}
		}
		t.Fatalf("no field of %s can satisfy its oneof rule", md.FullName())
	}
	for oneof := range listOf(md.Oneofs()) {
		if oneof.IsSynthetic() {
			continue
		}
		if bufBool(bufRules(oneof, g.bufOneof), "required") && m.WhichOneof(oneof) == nil {
			pick(slices.Collect(listOf(oneof.Fields())))
		}
		// A required member must be the one set.
		for fd := range listOf(oneof.Fields()) {
			if bufBool(g.fieldRules(fd), "required") {
				g.set(t, m, fd, depth, true)
			}
		}
	}
	rules := bufRules(md, g.bufMessage)
	if rules == nil {
		return
	}
	oneofRules := rules.Get(rules.Descriptor().Fields().ByName("oneof")).List()
	for i := range oneofRules.Len() {
		rule := oneofRules.Get(i).Message()
		names := rule.Get(rule.Descriptor().Fields().ByName("fields")).List()
		var fields []protoreflect.FieldDescriptor
		for j := range names.Len() {
			fields = append(fields, md.Fields().ByName(protoreflect.Name(names.Get(j).String())))
		}
		var set []protoreflect.FieldDescriptor
		for _, fd := range fields {
			if m.Has(fd) {
				set = append(set, fd)
			}
		}
		for _, fd := range set[min(1, len(set)):] {
			m.Clear(fd)
		}
		if len(set) == 0 && bufBool(rule, "required") {
			pick(fields)
		}
	}
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
	if bufBool(g.fieldRules(fd), "required") || !inOneof && g.opts.GetRespectFieldBehavior() && g.hasBehavior(fd, "REQUIRED") {
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
	if rules := g.fieldRules(fd); rules != nil && rules.WhichOneof(rules.Descriptor().Oneofs().ByName("type")) != nil {
		return true
	}
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

// fieldRules returns the protovalidate FieldRules of fd unless they are ignored.
func (g *generator) fieldRules(fd protoreflect.FieldDescriptor) protoreflect.Message {
	rules := bufRules(fd, g.bufField)
	if rules == nil {
		return nil
	}
	if ignore := rules.Descriptor().Fields().ByName("ignore"); rules.Has(ignore) && rules.Get(ignore).Enum() == 3 { // IGNORE_ALWAYS
		return nil
	}
	return rules
}

// bufRules returns the protovalidate option xd of desc: a known extension
// once protovalidate-go is linked, else parsed from the unknown fields.
func bufRules(desc protoreflect.Descriptor, xd protoreflect.ExtensionDescriptor) protoreflect.Message {
	if xd == nil {
		return nil
	}
	opts := desc.Options().ProtoReflect()
	var known protoreflect.Message
	opts.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.FullName() == xd.FullName() {
			known = v.Message()
		}
		return known == nil
	})
	if known != nil {
		return known
	}
	var raw []byte
	b := opts.GetUnknown()
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		m := protowire.ConsumeFieldValue(num, typ, b[n:])
		if num == xd.Number() && typ == protowire.BytesType {
			v, _ := protowire.ConsumeBytes(b[n : n+m])
			raw = append(raw, v...)
		}
		b = b[n+m:]
	}
	if raw == nil {
		return nil
	}
	msg := dynamicpb.NewMessage(xd.Message())
	if err := proto.Unmarshal(raw, msg); err != nil {
		panic(err)
	}
	return msg
}

func bufBool(m protoreflect.Message, name protoreflect.Name) bool {
	v, ok := ruleVal(m, name)
	return ok && v.Bool()
}

func listOf[T any](list interface {
	Len() int
	Get(int) T
}) func(func(T) bool) {
	return func(yield func(T) bool) {
		for i := range list.Len() {
			if !yield(list.Get(i)) {
				return
			}
		}
	}
}
