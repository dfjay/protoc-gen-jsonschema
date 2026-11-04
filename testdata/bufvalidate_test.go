package testdata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/big"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/modules"
	"github.com/dop251/goja"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"pgregory.net/rapid"
	"sigs.k8s.io/yaml"
)

// TestProtovalidate checks that the schema accepts the ProtoJSON of a message
// exactly when protovalidate accepts the message. Values come from the rules'
// constants and their neighbours, on both sides of every limit.
func TestProtovalidate(t *testing.T) {
	optionsImportPath, err := filepath.Abs(optionsDir)
	if err != nil {
		t.Fatal(err)
	}
	forEachCase(t, func(t *testing.T, tc *Testcase, dirPath string) {
		if !tc.CheckProtovalidate {
			t.Skip("checkProtovalidate is not set")
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
				o := &oracle{generator: g, classes: map[any]classified{}}
				message := g.entrypoint(t, file.GetContent(), file.GetName())
				properties := propertySchemas(t, file.GetName(), file.GetContent(), message, o.fieldNames(message))
				// How often protovalidate rejected and accepted each field.
				rejected, accepted := map[string]int{}, map[string]int{}
				rapid.Check(t, func(rt *rapid.T) {
					m := o.message(rt, message, 0)
					// Without EmitDefaultValues, a zero field without presence is
					// absent, and protovalidate still checks it.
					out, err := protojson.MarshalOptions{
						Resolver:          g.types,
						UseProtoNames:     g.opts.GetPreserveProtoFieldNames(),
						EmitDefaultValues: rapid.Bool().Draw(rt, "emitDefaultValues"),
					}.Marshal(m)
					if err != nil {
						rt.Fatalf("marshal: %v", err)
					}
					// Compare field by field, so that an invalid field does not
					// hide a wrong mapping of another.
					want, err := violations(m)
					if err != nil {
						rt.Fatalf("protovalidate: %v", err)
					}
					doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
					if err != nil {
						rt.Fatal(err)
					}
					if !g.opts.GetRespectProtojsonInt64() {
						// The schema describes 64-bit integers as numbers.
						doc = o.int64Numbers(message, doc)
					}
					got := map[string]bool{}
					if verr, ok := schema.Validate(doc).(*jsonschema.ValidationError); ok {
						rootErrors(verr, o.fieldNames(message), got)
					}
					for property, value := range doc.(map[string]any) {
						if properties[property].Validate(value) != nil {
							got[o.fieldNames(message)[property]] = true
						}
					}
					for fd := range listOf(message.Fields()) {
						if want[string(fd.Name())] {
							rejected[string(fd.Name())]++
						} else {
							accepted[string(fd.Name())]++
						}
					}
					want, got = o.normalize(message, want), o.normalize(message, got)
					names := maps.Clone(want)
					maps.Copy(names, got)
					for name := range names {
						if want[name] != got[name] {
							rt.Fatalf("%s: protovalidate violated %v, schema rejected %v\n%s\nprotovalidate: %v\nschema: %v",
								name, want[name], got[name], out, validator.Validate(m), schema.Validate(doc))
						}
					}
				})
				// Without both outcomes, a field's mapping went untested, where
				// its candidates allow both.
				for fd := range listOf(message.Fields()) {
					if g.fieldRules(fd) == nil || o.hasFormat(fd) {
						continue
					}
					canAccept, canReject := o.outcomes(fd)
					if name := string(fd.Name()); canReject && rejected[name] == 0 || canAccept && accepted[name] == 0 {
						t.Errorf("%s: protovalidate rejected it %d times and accepted it %d times", name, rejected[name], accepted[name])
					}
				}
			})
		}
	})
}

// validator checks rules with CEL: the native rules of protovalidate-go
// panic on wrapper list items of dynamic messages.
var validator = func() protovalidate.Validator {
	v, err := protovalidate.New(protovalidate.WithDisableNativeRules())
	if err != nil {
		panic(err)
	}
	return v
}()

// messageLevel stands for violations of the message as a whole, such as
// its oneof rules.
const messageLevel = ""

// violations returns the top-level fields of m that protovalidate rejects.
func violations(m protoreflect.ProtoMessage) (map[string]bool, error) {
	err := validator.Validate(m)
	var verr *protovalidate.ValidationError
	if err != nil && !errors.As(err, &verr) {
		return nil, err
	}
	out := map[string]bool{}
	if verr == nil {
		return out, nil
	}
	fields := m.ProtoReflect().Descriptor().Fields()
	for _, v := range verr.Violations {
		name := messageLevel
		if elements := v.Proto.GetField().GetElements(); len(elements) > 0 && fields.ByName(protoreflect.Name(elements[0].GetFieldName())) != nil {
			name = elements[0].GetFieldName()
		}
		out[name] = true
	}
	return out, nil
}

// rootErrors marks what errors at the root are about: the missing required
// fields, or the message for oneOf, anyOf and not. Errors inside properties
// are left to their own schemas: the validator can misplace them.
func rootErrors(err *jsonschema.ValidationError, fields map[string]string, out map[string]bool) {
	if len(err.InstanceLocation) > 0 {
		return
	}
	switch k := err.ErrorKind.(type) {
	case *kind.Required:
		for _, property := range k.Missing {
			out[fields[property]] = true
		}
		return
	case *kind.OneOf, *kind.AnyOf, *kind.Not:
		out[messageLevel] = true
		return
	}
	for _, cause := range err.Causes {
		rootErrors(cause, fields, out)
	}
}

// propertySchemas compiles the schema of each property of message, from the
// same document.
func propertySchemas(t *testing.T, name, content string, message protoreflect.MessageDescriptor, fields map[string]string) map[string]*jsonschema.Schema {
	t.Helper()
	data := []byte(content)
	if ext := filepath.Ext(name); ext == ".yaml" || ext == ".yml" {
		var err error
		if data, err = yaml.YAMLToJSON(data); err != nil {
			t.Fatal(err)
		}
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defs := "$defs"
	if _, ok := doc.(map[string]any)["definitions"]; ok {
		defs = "definitions"
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	url := "file:///" + name
	if err := c.AddResource(url, doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]*jsonschema.Schema{}
	for property := range fields {
		pointer := strings.Join([]string{defs, "." + string(message.FullName()), "properties", property}, "/")
		if out[property], err = c.Compile(url + "#/" + pointer); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// fieldNames maps the JSON names of the fields of md to their proto names.
func (g *generator) fieldNames(md protoreflect.MessageDescriptor) map[string]string {
	out := map[string]string{}
	for fd := range listOf(md.Fields()) {
		name := fd.JSONName()
		if g.opts.GetPreserveProtoFieldNames() {
			name = string(fd.Name())
		}
		out[name] = string(fd.Name())
	}
	return out
}

// normalize makes both sides comparable. Oneofs and their members are
// about the message: protovalidate reports a oneof by its name, the schema
// by its members. Formats are left out: JSON Schema defines them as
// protovalidate does, but validators check them loosely.
func (o *oracle) normalize(md protoreflect.MessageDescriptor, rejected map[string]bool) map[string]bool {
	out := map[string]bool{}
	for name, v := range rejected {
		if !v {
			continue
		}
		if oneof := md.Oneofs().ByName(protoreflect.Name(name)); oneof != nil {
			name = messageLevel
		}
		if fd := md.Fields().ByName(protoreflect.Name(name)); fd != nil {
			if oneof := fd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
				name = messageLevel
			}
			if o.hasFormat(fd) {
				continue
			}
		}
		out[name] = true
	}
	return out
}

func (o *oracle) hasFormat(fd protoreflect.FieldDescriptor) bool {
	str := typedRules(bufRules(fd, o.bufField), protoreflect.StringKind)
	return slices.ContainsFunc([]protoreflect.Name{"ip", "ipv6", "uri", "uri_ref"}, func(name protoreflect.Name) bool {
		return bufBool(str, name)
	})
}

// oracle builds messages whose rule-constrained fields take values near
// their limits.
type oracle struct {
	*generator
	classes map[any]classified
}

func (o *oracle) message(t *rapid.T, md protoreflect.MessageDescriptor, depth int) *dynamicpb.Message {
	m := dynamicpb.NewMessage(md)
	for fd := range listOf(md.Fields()) {
		if o.hidden(fd) || !rapid.Bool().Draw(t, string(fd.Name())) {
			continue
		}
		rules := bufRules(fd, o.bufField)
		switch {
		case fd.IsMap():
			mapRules := ruleMsg(rules, "map")
			keys := o.classify(fd, roleKey, o.pool(fd.MapKey(), ruleMsg(mapRules, "keys")))
			values := o.classify(fd, roleValue, o.pool(fd.MapValue(), ruleMsg(mapRules, "values")))
			if keys.empty() || values.empty() && fd.MapValue().Message() == nil {
				continue
			}
			mp := m.Mutable(fd).Map()
			for range o.size(t, mapRules, "max_pairs") {
				mp.Set(keys.draw(t, "key").MapKey(), o.value(t, fd.MapValue(), values, depth))
			}
		case fd.IsList():
			repeated := ruleMsg(rules, "repeated")
			items := o.classify(fd, roleItem, o.pool(fd, ruleMsg(repeated, "items")))
			if items.empty() && fd.Message() == nil {
				continue
			}
			list := m.Mutable(fd).List()
			for range o.size(t, repeated, "max_items") {
				list.Append(o.value(t, fd, items, depth))
			}
		default:
			values := o.classify(fd, roleField, o.pool(fd, rules))
			if values.empty() && (fd.Message() == nil || depth >= 2) {
				continue
			}
			m.Set(fd, o.value(t, fd, values, depth))
		}
	}
	return m
}

// outcomes reports whether the classified candidates of fd hold values
// protovalidate accepts and values it rejects. Messages are not classified.
func (o *oracle) outcomes(fd protoreflect.FieldDescriptor) (canAccept, canReject bool) {
	for k, c := range o.classes {
		if k.(classKey).field == fd.FullName() {
			canAccept = canAccept || len(c.valid) > 0
			canReject = canReject || len(c.invalid) > 0
		}
	}
	return canAccept, canReject
}

// int64Numbers turns the 64-bit integers that protojson writes as strings
// into JSON numbers.
func (o *oracle) int64Numbers(md protoreflect.MessageDescriptor, doc any) any {
	obj, ok := doc.(map[string]any)
	if !ok {
		return doc
	}
	names := o.fieldNames(md)
	for property, value := range obj {
		fd := md.Fields().ByName(protoreflect.Name(names[property]))
		if fd == nil {
			continue
		}
		convert := func(fd protoreflect.FieldDescriptor, v any) any {
			kind := fd.Kind()
			if md := fd.Message(); md != nil && isWrapper(md) {
				kind = md.Fields().ByName("value").Kind()
			}
			if s, ok := v.(string); ok && isInt64Kind(kind) {
				return json.Number(s)
			}
			if fd.Message() != nil && !isWrapper(fd.Message()) {
				return o.int64Numbers(fd.Message(), v)
			}
			return v
		}
		switch {
		case fd.IsList():
			if list, ok := value.([]any); ok {
				for i := range list {
					list[i] = convert(fd, list[i])
				}
			}
		case fd.IsMap():
			if entries, ok := value.(map[string]any); ok {
				for k, v := range entries {
					entries[k] = convert(fd.MapValue(), v)
				}
			}
		default:
			obj[property] = convert(fd, value)
		}
	}
	return obj
}

func isInt64Kind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

// size draws a collection size, mostly one, up to one more than its limit.
func (o *oracle) size(t *rapid.T, rules protoreflect.Message, name protoreflect.Name) int {
	limit := 3
	if v, ok := ruleVal(rules, name); ok {
		limit = min(int(v.Uint())+1, 6)
	}
	return rapid.SampledFrom([]int{0, 1, 1, 1, 2, limit}).Draw(t, "size")
}

func (o *oracle) value(t *rapid.T, fd protoreflect.FieldDescriptor, values classified, depth int) protoreflect.Value {
	if fd.Message() != nil && !isWrapper(fd.Message()) {
		return protoreflect.ValueOfMessage(o.message(t, fd.Message(), depth+1))
	}
	return values.draw(t, string(fd.Name()))
}

// The roles of a value in a field, which classify tells apart.
type role int

const (
	roleField role = iota
	roleItem
	roleKey
	roleValue
)

type classKey struct {
	field protoreflect.FullName
	role  role
}

// classified holds the candidate values of one role of a field, by whether
// protovalidate accepts them. Draws pick either half evenly.
type classified struct {
	valid, invalid []protoreflect.Value
}

func (c classified) empty() bool { return len(c.valid)+len(c.invalid) == 0 }

func (c classified) draw(t *rapid.T, label string) protoreflect.Value {
	if len(c.invalid) == 0 || len(c.valid) > 0 && rapid.Bool().Draw(t, label+" valid") {
		return rapid.SampledFrom(c.valid).Draw(t, label)
	}
	return rapid.SampledFrom(c.invalid).Draw(t, label)
}

// classify asks protovalidate about each candidate alone: in a message with
// only fd set, as the field, a one-item list or a one-entry map.
func (o *oracle) classify(fd protoreflect.FieldDescriptor, r role, candidates []protoreflect.Value) classified {
	if c, ok := o.classes[classKey{fd.FullName(), r}]; ok {
		return c
	}
	var c classified
	for _, v := range candidates {
		m := dynamicpb.NewMessage(fd.ContainingMessage())
		switch r {
		case roleField:
			m.Set(fd, v)
		case roleItem:
			m.Mutable(fd).List().Append(v)
		case roleKey:
			value := fd.MapValue().Default()
			if fd.MapValue().Message() != nil {
				value = m.Mutable(fd).Map().NewValue()
			}
			m.Mutable(fd).Map().Set(v.MapKey(), value)
		case roleValue:
			m.Mutable(fd).Map().Set(fd.MapKey().Default().MapKey(), v)
		}
		if o.accepts(m, fd, r) {
			c.valid = append(c.valid, v)
		} else {
			c.invalid = append(c.invalid, v)
		}
	}
	o.classes[classKey{fd.FullName(), r}] = c
	return c
}

// accepts reports whether protovalidate finds no violation of the role of
// fd in m.
func (o *oracle) accepts(m *dynamicpb.Message, fd protoreflect.FieldDescriptor, r role) bool {
	var verr *protovalidate.ValidationError
	if !errors.As(validator.Validate(m), &verr) {
		return true
	}
	for _, v := range verr.Violations {
		elements := v.Proto.GetField().GetElements()
		if len(elements) == 0 || elements[0].GetFieldName() != string(fd.Name()) {
			continue
		}
		element := elements[0].GetSubscript() != nil || len(elements) > 1
		switch {
		case r == roleField,
			r == roleItem && element,
			r == roleKey && v.Proto.GetForKey(),
			r == roleValue && element && !v.Proto.GetForKey():
			return false
		}
	}
	return true
}

// pool returns candidate values for fd from the constants of its rules.
func (o *oracle) pool(fd protoreflect.FieldDescriptor, fieldRules protoreflect.Message) []protoreflect.Value {
	if md := fd.Message(); md != nil {
		if !isWrapper(md) {
			return nil
		}
		value := md.Fields().ByName("value")
		var out []protoreflect.Value
		for _, v := range o.pool(value, fieldRules) {
			w := dynamicpb.NewMessage(md)
			w.Set(value, v)
			out = append(out, protoreflect.ValueOfMessage(w))
		}
		return out
	}

	// In field order: Range has none, and rapid needs the same pool each run.
	typed := typedRules(fieldRules, fd.Kind())
	var constants []protoreflect.Value
	if typed != nil {
		for rule := range listOf(typed.Descriptor().Fields()) {
			if !typed.Has(rule) || rule.Message() != nil {
				continue
			}
			v := typed.Get(rule)
			if rule.IsList() {
				for i := range v.List().Len() {
					constants = append(constants, v.List().Get(i))
				}
			} else {
				constants = append(constants, v)
			}
		}
	}

	switch fd.Kind() {
	case protoreflect.BoolKind:
		return []protoreflect.Value{protoreflect.ValueOfBool(false), protoreflect.ValueOfBool(true)}
	case protoreflect.EnumKind:
		var out []protoreflect.Value
		for v := range listOf(fd.Enum().Values()) {
			out = append(out, protoreflect.ValueOfEnum(v.Number()))
		}
		// Canonical accepts declared names only, so an undefined number
		// only where defined_only rejects it too.
		if v, ok := ruleVal(typed, "defined_only"); ok && v.Bool() {
			out = append(out, protoreflect.ValueOfEnum(99))
		}
		return out
	case protoreflect.StringKind:
		return stringPool(constants)
	case protoreflect.BytesKind, protoreflect.MessageKind, protoreflect.GroupKind:
		return nil
	}
	return numberPool(fd.Kind(), constants)
}

func isWrapper(md protoreflect.MessageDescriptor) bool {
	return md.ParentFile().Package() == "google.protobuf" && strings.HasSuffix(string(md.Name()), "Value") &&
		md.Name() != "Value" && md.Name() != "ListValue"
}

// hostname253 is a hostname of the most characters protovalidate allows.
var hostname253 = strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("a", 61)

// stringPool has strings of every length near a length rule, the rule's
// strings and combinations of them, and samples of each format.
func stringPool(constants []protoreflect.Value) []protoreflect.Value {
	pool := []string{"", "a", "ab", "abc", "a-b.z", "admin",
		"a@b.co", ".a@example.com", "not an email", "host.example", "-bad-",
		"1.2.3.4", "::1", "999.1.1.1", "255.255.255.255", "01.2.3.4", "1.2.3",
		"host.example.", "example.123", "a-.example", "x." + strings.Repeat("a", 64),
		hostname253, hostname253 + ".", hostname253 + "a",
		"https://example.com/x", "/relative", "a b",
		"123e4567-e89b-12d3-a456-426614174000", "123E4567-E89B-12D3-A456-426614174000", "123e4567",
		"abc", "a\rc", "a\nc", "AB", "aB", "\U0001F600", "\U0001F600\U0001F600"}
	var strs []string
	for _, c := range constants {
		switch x := c.Interface().(type) {
		case string:
			strs = append(strs, x)
		case uint64:
			for n := max(int(x)-1, 0); n <= min(int(x)+1, 70); n++ {
				pool = append(pool, strings.Repeat("x", n))
			}
		}
	}
	pool = append(pool, strs...)
	for _, a := range strs {
		for _, b := range strs {
			pool = append(pool, a+b, a+"-"+b)
		}
	}
	var out []protoreflect.Value
	for _, s := range pool {
		out = append(out, protoreflect.ValueOfString(s))
	}
	return out
}

// numberPool has each constant and its neighbours within the range of kind.
func numberPool(kind protoreflect.Kind, constants []protoreflect.Value) []protoreflect.Value {
	if kind == protoreflect.FloatKind || kind == protoreflect.DoubleKind {
		floats := []float64{0, 1, -1, math.NaN(), math.Inf(1), math.Inf(-1)}
		for _, c := range constants {
			if f, ok := c.Interface().(float64); ok {
				floats = append(floats, f, f-1, f+1, f-0.5, f+0.5, math.Nextafter(f, math.Inf(1)), math.Nextafter(f, math.Inf(-1)))
			} else if f, ok := c.Interface().(float32); ok {
				x := float64(f)
				floats = append(floats, x, x-1, x+1, x-0.5, x+0.5,
					float64(math.Nextafter32(f, float32(math.Inf(1)))), float64(math.Nextafter32(f, float32(math.Inf(-1)))))
			}
		}
		var out []protoreflect.Value
		for _, f := range floats {
			if kind == protoreflect.FloatKind {
				out = append(out, protoreflect.ValueOfFloat32(float32(f)))
			} else {
				out = append(out, protoreflect.ValueOfFloat64(f))
			}
		}
		return out
	}

	ints := []*big.Int{big.NewInt(0), big.NewInt(1), big.NewInt(-1)}
	for _, c := range constants {
		var x *big.Int
		switch v := c.Interface().(type) {
		case int32:
			x = big.NewInt(int64(v))
		case int64:
			x = big.NewInt(v)
		case uint32:
			x = new(big.Int).SetUint64(uint64(v))
		case uint64:
			x = new(big.Int).SetUint64(v)
		default:
			continue
		}
		ints = append(ints, x, new(big.Int).Sub(x, big.NewInt(1)), new(big.Int).Add(x, big.NewInt(1)))
	}
	var lo, hi *big.Int
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		lo, hi = big.NewInt(math.MinInt32), big.NewInt(math.MaxInt32)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		lo, hi = big.NewInt(math.MinInt64), big.NewInt(math.MaxInt64)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		lo, hi = big.NewInt(0), big.NewInt(math.MaxUint32)
	default:
		lo, hi = big.NewInt(0), new(big.Int).SetUint64(math.MaxUint64)
	}
	var out []protoreflect.Value
	for _, x := range ints {
		if x.Cmp(lo) < 0 || x.Cmp(hi) > 0 {
			continue
		}
		switch kind {
		case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
			out = append(out, protoreflect.ValueOfInt32(int32(x.Int64())))
		case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
			out = append(out, protoreflect.ValueOfInt64(x.Int64()))
		case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
			out = append(out, protoreflect.ValueOfUint32(uint32(x.Uint64())))
		default:
			out = append(out, protoreflect.ValueOfUint64(x.Uint64()))
		}
	}
	return out
}

// typedRules returns the rules of FieldRules for a value of kind.
func typedRules(rules protoreflect.Message, kind protoreflect.Kind) protoreflect.Message {
	return ruleMsg(rules, protoreflect.Name(kind.String()))
}

func ruleMsg(m protoreflect.Message, name protoreflect.Name) protoreflect.Message {
	if v, ok := ruleVal(m, name); ok {
		if msg, ok := v.Interface().(protoreflect.Message); ok {
			return msg
		}
	}
	return nil
}

func ruleVal(m protoreflect.Message, name protoreflect.Name) (protoreflect.Value, bool) {
	if m == nil {
		return protoreflect.Value{}, false
	}
	fd := m.Descriptor().Fields().ByName(name)
	if fd == nil || !m.Has(fd) {
		return protoreflect.Value{}, false
	}
	return m.Get(fd), true
}

// TestPatterns checks the translation of protovalidate's RE2 patterns with
// an ECMA-262 engine, as JSON Schema validators in JavaScript run them: on
// sample strings, the translated pattern matches exactly where RE2 does.
func TestPatterns(t *testing.T) {
	optionsImportPath, err := filepath.Abs(optionsDir)
	if err != nil {
		t.Fatal(err)
	}
	forEachCase(t, func(t *testing.T, tc *Testcase, dirPath string) {
		if !tc.CheckProtovalidate {
			t.Skip("checkProtovalidate is not set")
		}
		req := buildRequest(t, tc, dirPath, optionsImportPath)
		resp := modules.Generate(req, nil, false)
		g := newGenerator(t, req)
		vm := goja.New()
		for _, file := range resp.File {
			var schema struct {
				Defs        map[string]struct{ Pattern string } `json:"$defs"`
				Definitions map[string]struct{ Pattern string } `json:"definitions"`
			}
			if err := json.Unmarshal([]byte(file.GetContent()), &schema); err != nil {
				t.Fatal(err)
			}
			defs := schema.Defs
			if defs == nil {
				defs = schema.Definitions
			}
			message := g.entrypoint(t, file.GetContent(), file.GetName())
			for fd := range listOf(message.Fields()) {
				v, ok := ruleVal(typedRules(bufRules(fd, g.bufField), protoreflect.StringKind), "pattern")
				if !ok {
					continue
				}
				translated := defs["."+string(fd.FullName())].Pattern
				if translated == "" {
					t.Logf("%s: %q has no translation", fd.Name(), v.String())
					continue
				}
				re2 := regexp.MustCompile(v.String())
				ecma, err := vm.RunString(fmt.Sprintf("new RegExp(%q, 'u')", translated))
				if err != nil {
					t.Fatalf("%s: %q is not ECMA-262: %v", fd.Name(), translated, err)
				}
				test, _ := goja.AssertFunction(ecma.ToObject(vm).Get("test"))
				for _, s := range stringPool(nil) {
					got, err := test(ecma, vm.ToValue(s.String()))
					if err != nil {
						t.Fatal(err)
					}
					if want := re2.MatchString(s.String()); got.ToBoolean() != want {
						t.Errorf("%s: %q on %q: RE2 %v, ECMA-262 %q %v", fd.Name(), v.String(), s.String(), want, translated, got.ToBoolean())
					}
				}
			}
		}
	})
}

// TestIPFormats checks the ip and ipv6 rules, which the oracle leaves out
// with the other formats, on addresses where format ipv6 and protovalidate
// agree, and on addresses with a zone, which only protovalidate accepts.
func TestIPFormats(t *testing.T) {
	optionsImportPath, err := filepath.Abs(optionsDir)
	if err != nil {
		t.Fatal(err)
	}
	samples := []string{
		"", "1.2.3.4", "256.1.1.1", "01.2.3.4", "::", "::1", "fe80::1", "1:2:3:4:5:6:7:8",
		"1:2:3:4:5:6:7:8:9", "1::2::3", "12345::", "::ffff:1.2.3.4", "::1/64", "gggg::1",
		"fe80::1%eth0", "fe80::1%25en1", "::%x", "fe80::1%", "1.2.3.4%eth0", "::ffff:1.2.3.4%1",
		"1:2:3:4:5:6:7:8%z", "1:2:3:4:5:6:7:8:9%z", "1::2::3%z", "12345::%z", "%eth0",
	}
	forEachCase(t, func(t *testing.T, tc *Testcase, dirPath string) {
		if !tc.CheckProtovalidate {
			t.Skip("checkProtovalidate is not set")
		}
		req := buildRequest(t, tc, dirPath, optionsImportPath)
		resp := modules.Generate(req, nil, false)
		if resp.Error != nil {
			t.Fatal(resp.GetError())
		}
		g := newGenerator(t, req)
		o := &oracle{generator: g}
		for _, file := range resp.File {
			message := g.entrypoint(t, file.GetContent(), file.GetName())
			fields := o.fieldNames(message)
			properties := propertySchemas(t, file.GetName(), file.GetContent(), message, fields)
			for property, name := range fields {
				fd := message.Fields().ByName(protoreflect.Name(name))
				str := typedRules(bufRules(fd, g.bufField), protoreflect.StringKind)
				if !bufBool(str, "ip") && !bufBool(str, "ipv6") {
					continue
				}
				for _, s := range samples {
					m := dynamicpb.NewMessage(message)
					m.Set(fd, protoreflect.ValueOfString(s))
					want, err := violations(m)
					if err != nil {
						t.Fatal(err)
					}
					if got := properties[property].Validate(s) != nil; got != want[name] {
						t.Errorf("%s: %q: protovalidate violated %v, schema rejected %v", name, s, want[name], got)
					}
				}
			}
		}
	})
}
