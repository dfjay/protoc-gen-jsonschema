package modules

import (
	"cmp"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/jsonschema"
	"github.com/dfjay/protoc-gen-jsonschema/pkg/proto"
	"google.golang.org/protobuf/encoding/protowire"
	goproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// bufValidate reads protovalidate rules from the request's descriptors
// instead of linking protovalidate.
type bufValidate struct {
	field, oneof, message protoreflect.ExtensionDescriptor
}

func newBufValidate(files *protoregistry.Files) bufValidate {
	find := func(name, options protoreflect.FullName) protoreflect.ExtensionDescriptor {
		d, err := files.FindDescriptorByName(name)
		if err != nil {
			return nil
		}
		xd, ok := d.(protoreflect.ExtensionDescriptor)
		if !ok || xd.Message() == nil || xd.ContainingMessage().FullName() != options {
			return nil
		}
		return xd
	}
	return bufValidate{
		field:   find("buf.validate.field", "google.protobuf.FieldOptions"),
		oneof:   find("buf.validate.oneof", "google.protobuf.OneofOptions"),
		message: find("buf.validate.message", "google.protobuf.MessageOptions"),
	}
}

// Values of buf.validate.Ignore.
const (
	ignoreIfZeroValue = 1
	ignoreAlways      = 3
)

// unvalidatedPrefix starts the keys of schemas built without protovalidate
// rules; other keys start with a dot.
const unvalidatedPrefix = "unvalidated"

// fieldRules returns the FieldRules of field, or nil if none or ignored.
func (b bufValidate) fieldRules(field protoreflect.FieldDescriptor) protoreflect.Message {
	rules := extensionMessage(field, b.field)
	if rules != nil && !ruleHas(rules, "ignore") && slices.ContainsFunc(b.messageOneofs(field.ContainingMessage()), func(r messageOneofRule) bool {
		return slices.Contains(r.fields, field.Name())
	}) {
		// Fields of a message oneof rule default to IGNORE_IF_ZERO_VALUE.
		rules = goproto.Clone(rules.Interface()).ProtoReflect()
		rules.Set(rules.Descriptor().Fields().ByName("ignore"), protoreflect.ValueOfEnum(ignoreIfZeroValue))
	}
	return activeRules(rules)
}

// requiresValue reports whether protovalidate rejects field when absent:
// it is required, or has no presence and the rules reject its zero value.
// The required field option and options that replace the rules come first.
func (b bufValidate) requiresValue(field protoreflect.FieldDescriptor) bool {
	fo := proto.GetFieldOptions(field)
	rules := b.fieldRules(field)
	switch {
	case fo != nil && fo.Required != nil:
		return false
	case ruleBool(rules, "required"):
		return true
	case rules == nil || field.HasPresence() || ignoresZero(rules):
		return false
	case field.IsList():
		return fo.GetArray() == nil && ruleUint(ruleMessage(rules, "repeated"), "min_items") > 0
	case field.IsMap():
		return ruleUint(ruleMessage(rules, "map"), "min_pairs") > 0
	}
	switch typed, kind := typeRules(rules, field.Kind()), field.Kind(); {
	case typed == nil:
		return false
	case kind == protoreflect.EnumKind:
		return !enumAccepts(typed, field.Enum(), field.Default().Enum())
	case kind == protoreflect.BoolKind:
		return ruleBool(typed, "const")
	case kind == protoreflect.StringKind:
		return fo.GetString_() == nil && !acceptsEmptyString(typed)
	case fo.GetNumeric() != nil:
		return false
	case kind == protoreflect.FloatKind, kind == protoreflect.DoubleKind:
		return !floatAccepts(typed, 0)
	default:
		return !intAccepts(typed, new(big.Float))
	}
}

// ignoresAlways returns the message of field, or of its items or map values,
// if IGNORE_ALWAYS covers it and it has rules.
func (b bufValidate) ignoresAlways(field protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	rules := extensionMessage(field, b.field)
	message, element := field.Message(), ruleMessage(ruleMessage(rules, "repeated"), "items")
	if field.IsMap() {
		message, element = field.MapValue().Message(), ruleMessage(ruleMessage(rules, "map"), "values")
	}
	if message == nil || ignore(rules) != ignoreAlways && ignore(element) != ignoreAlways ||
		!b.hasRules(message, map[protoreflect.FullName]bool{}) {
		return nil
	}
	return message
}

// hasRules reports whether message or a message it holds has rules.
func (b bufValidate) hasRules(message protoreflect.MessageDescriptor, seen map[protoreflect.FullName]bool) bool {
	if seen[message.FullName()] {
		return false
	}
	seen[message.FullName()] = true
	if extensionMessage(message, b.message) != nil {
		return true
	}
	for oneof := range listOf(message.Oneofs()) {
		if extensionMessage(oneof, b.oneof) != nil {
			return true
		}
	}
	for field := range listOf(message.Fields()) {
		if extensionMessage(field, b.field) != nil {
			return true
		}
		inner := field.Message()
		if field.IsMap() {
			inner = field.MapValue().Message()
		}
		if inner != nil && b.hasRules(inner, seen) {
			return true
		}
	}
	return false
}

type messageOneofRule struct {
	fields   []protoreflect.Name
	required bool
}

// messageOneofs returns the (buf.validate.message).oneof rules of message.
func (b bufValidate) messageOneofs(message protoreflect.MessageDescriptor) []messageOneofRule {
	var out []messageOneofRule
	for _, v := range ruleList(extensionMessage(message, b.message), "oneof") {
		rule := v.Message()
		var names []protoreflect.Name
		for _, name := range ruleList(rule, "fields") {
			names = append(names, protoreflect.Name(name.String()))
		}
		out = append(out, messageOneofRule{fields: names, required: ruleBool(rule, "required")})
	}
	return out
}

// extensionMessage returns the message option xd of desc, or nil.
func extensionMessage(desc protoreflect.Descriptor, xd protoreflect.ExtensionDescriptor) protoreflect.Message {
	if xd == nil {
		return nil
	}
	opts := desc.Options().ProtoReflect()
	// A known extension, if a linked package registered it.
	var found protoreflect.Message
	opts.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.FullName() == xd.FullName() {
			found = v.Message()
		}
		return found == nil
	})
	if found != nil {
		return found
	}

	// Repeated occurrences of a message field merge, as in their concatenation.
	var raw []byte
	for unknown := opts.GetUnknown(); len(unknown) > 0; {
		num, typ, n := protowire.ConsumeTag(unknown)
		if n < 0 {
			return nil
		}
		m := protowire.ConsumeFieldValue(num, typ, unknown[n:])
		if m < 0 {
			return nil
		}
		if num == xd.Number() && typ == protowire.BytesType {
			value, _ := protowire.ConsumeBytes(unknown[n : n+m])
			raw = append(raw, value...)
		}
		unknown = unknown[n+m:]
	}
	if raw == nil {
		return nil
	}
	msg := dynamicpb.NewMessage(xd.Message())
	if err := goproto.Unmarshal(raw, msg); err != nil {
		return nil
	}
	return msg
}

func ruleHas(m protoreflect.Message, name protoreflect.Name) bool {
	if m == nil {
		return false
	}
	fd := m.Descriptor().Fields().ByName(name)
	return fd != nil && m.Has(fd)
}

func ruleGet(m protoreflect.Message, name protoreflect.Name) protoreflect.Value {
	return m.Get(m.Descriptor().Fields().ByName(name))
}

func ruleBool(m protoreflect.Message, name protoreflect.Name) bool {
	return ruleHas(m, name) && ruleGet(m, name).Bool()
}

func ruleUint(m protoreflect.Message, name protoreflect.Name) uint64 {
	if !ruleHas(m, name) {
		return 0
	}
	return ruleGet(m, name).Uint()
}

// ruleSize reads a size rule as a JSON Schema length.
func ruleSize(m protoreflect.Message, name protoreflect.Name) *int {
	return new(int(min(ruleUint(m, name), math.MaxInt32)))
}

func ruleMessage(m protoreflect.Message, name protoreflect.Name) protoreflect.Message {
	if !ruleHas(m, name) {
		return nil
	}
	return ruleGet(m, name).Message()
}

func ruleList(m protoreflect.Message, name protoreflect.Name) []protoreflect.Value {
	if !ruleHas(m, name) {
		return nil
	}
	list := ruleGet(m, name).List()
	out := make([]protoreflect.Value, list.Len())
	for i := range out {
		out[i] = list.Get(i)
	}
	return out
}

func ignore(rules protoreflect.Message) protoreflect.EnumNumber {
	if !ruleHas(rules, "ignore") {
		return 0
	}
	return ruleGet(rules, "ignore").Enum()
}

func ignoresZero(rules protoreflect.Message) bool {
	return ignore(rules) == ignoreIfZeroValue
}

// activeRules returns FieldRules unless they set IGNORE_ALWAYS.
func activeRules(rules protoreflect.Message) protoreflect.Message {
	if ignore(rules) == ignoreAlways {
		return nil
	}
	return rules
}

// typeRules returns the rules of FieldRules for a value of kind. Bytes are
// base64 in JSON: their rules have no JSON Schema form.
func typeRules(rules protoreflect.Message, kind protoreflect.Kind) protoreflect.Message {
	name, ok := map[protoreflect.Kind]protoreflect.Name{
		protoreflect.FloatKind: "float", protoreflect.DoubleKind: "double",
		protoreflect.Int32Kind: "int32", protoreflect.Int64Kind: "int64",
		protoreflect.Uint32Kind: "uint32", protoreflect.Uint64Kind: "uint64",
		protoreflect.Sint32Kind: "sint32", protoreflect.Sint64Kind: "sint64",
		protoreflect.Fixed32Kind: "fixed32", protoreflect.Fixed64Kind: "fixed64",
		protoreflect.Sfixed32Kind: "sfixed32", protoreflect.Sfixed64Kind: "sfixed64",
		protoreflect.BoolKind: "bool", protoreflect.StringKind: "string",
		protoreflect.EnumKind: "enum",
	}[kind]
	if !ok {
		return nil
	}
	return ruleMessage(rules, name)
}

// hasNumericRules reports whether rules constrain a number.
func hasNumericRules(rules protoreflect.Message) bool {
	return slices.ContainsFunc([]protoreflect.Name{"gt", "gte", "lt", "lte", "const", "in", "not_in", "finite"}, func(name protoreflect.Name) bool {
		return ruleHas(rules, name)
	})
}

// float32Value is the shortest decimal of x, not of its float64 widening.
func float32Value(x float32) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(float64(x), 'g', -1, 32), 64)
	return f
}

// intValue returns an integer rule value exactly.
func intValue(v protoreflect.Value) *big.Float {
	switch v.Interface().(type) {
	case int32, int64:
		return new(big.Float).SetInt64(v.Int())
	}
	return new(big.Float).SetUint64(v.Uint())
}

// applyNumericRules adds numeric rules to schema, a JSON number or, with
// asString, a decimal string, which bounds cannot constrain. NaN and the
// infinities are left to floatSpecials.
func applyNumericRules(schema *jsonschema.Schema, rules protoreflect.Message, asString bool) {
	if rules == nil {
		return
	}
	// value returns the JSON form of a rule value, if it has one.
	value := func(v protoreflect.Value) (any, bool) {
		switch x := v.Interface().(type) {
		case float32:
			f := float32Value(x)
			return f, !math.IsNaN(f) && !math.IsInf(f, 0)
		case float64:
			return x, !math.IsNaN(x) && !math.IsInf(x, 0)
		case int64:
			if asString {
				return strconv.FormatInt(x, 10), true
			}
		case uint64:
			if asString {
				return strconv.FormatUint(x, 10), true
			}
		}
		return v.Interface(), true
	}
	// Bounds still narrow the values of const and in for a decimal string.
	allowed := func(v protoreflect.Value) bool {
		_, ok := value(v)
		return ok && (!asString || intAccepts(rules, intValue(v)))
	}
	var in, notIn []any
	for _, v := range ruleList(rules, "in") {
		if f, _ := value(v); allowed(v) {
			in = appendUnique(in, f)
		}
	}
	for _, v := range ruleList(rules, "not_in") {
		if f, ok := value(v); ok {
			notIn = appendUnique(notIn, f)
		}
	}
	if c := ruleGet(rules, "const"); ruleHas(rules, "const") && !allowed(c) {
		schema.AllOf = append(schema.AllOf, never())
	} else {
		f, _ := value(c)
		restrict(schema, f, ruleHas(rules, "const"), in, ruleHas(rules, "in"))
	}
	if len(notIn) > 0 {
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Not: &jsonschema.Schema{Enum: notIn}})
	}
	if asString {
		return
	}

	low, high := bound(rules, "gt", "gte", true), bound(rules, "lt", "lte", false)
	if inverted(low, high) {
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{AnyOf: []*jsonschema.Schema{low.schema(), high.schema()}})
		return
	}
	for _, b := range []numberBound{low, high} {
		// A bound replaces the range of the type, as from lenientInteger,
		// which holds the rule values.
		switch s := b.schema(); {
		case b.never:
			schema.AllOf = append(schema.AllOf, s)
		case b.value == nil:
		case b.lower:
			schema.Minimum, schema.ExclusiveMinimum = s.Minimum, s.ExclusiveMinimum
		default:
			schema.Maximum, schema.ExclusiveMaximum = s.Maximum, s.ExclusiveMaximum
		}
	}
}

// numberBound is one side of a range: none, a finite value, or never met,
// as by gt: inf. raw is the exact rule value, nil without one or for NaN.
type numberBound struct {
	name                    protoreflect.Name
	raw                     *big.Float
	value                   *float64
	exclusive, lower, never bool
}

func (b numberBound) schema() *jsonschema.Schema {
	switch {
	case b.never:
		return never()
	case b.value == nil:
		return &jsonschema.Schema{}
	case b.lower && b.exclusive:
		return &jsonschema.Schema{ExclusiveMinimum: b.value}
	case b.lower:
		return &jsonschema.Schema{Minimum: b.value}
	case b.exclusive:
		return &jsonschema.Schema{ExclusiveMaximum: b.value}
	}
	return &jsonschema.Schema{Maximum: b.value}
}

// bound reads one side of the range. An integer beyond 2^53, which a
// float64 cannot hold exactly, constrains nothing.
func bound(rules protoreflect.Message, exclusive, inclusive protoreflect.Name, lower bool) numberBound {
	name := exclusive
	if !ruleHas(rules, name) {
		if name = inclusive; !ruleHas(rules, name) {
			return numberBound{}
		}
	}
	b := numberBound{name: name, exclusive: name == exclusive, lower: lower}
	var f float64
	switch v := ruleGet(rules, name); x := v.Interface().(type) {
	case float32:
		f = float32Value(x)
	case float64:
		f = x
	default:
		var accuracy big.Accuracy
		b.raw = intValue(v)
		if f, accuracy = b.raw.Float64(); accuracy != big.Exact {
			return b
		}
	}
	if math.IsNaN(f) {
		b.never = true
		return b
	}
	b.raw = big.NewFloat(f)
	switch {
	case math.IsInf(f, 0) && (f > 0) == lower:
		// Nothing finite is beyond an infinite bound.
		b.never = true
	case !math.IsInf(f, 0):
		b.value = &f
	}
	return b
}

// inverted reports whether the upper bound is below the lower one, which
// protovalidate reads as outside the range.
func inverted(low, high numberBound) bool {
	return low.raw != nil && high.raw != nil && high.raw.Cmp(low.raw) < 0
}

// numberAccepts reports whether numeric rules accept a value, given how it
// compares with a rule value; ok is false if they are unordered, as NaN.
func numberAccepts(rules protoreflect.Message, compare func(protoreflect.Value) (c int, ok bool)) bool {
	equal := func(v protoreflect.Value) bool {
		c, ok := compare(v)
		return ok && c == 0
	}
	if ruleHas(rules, "const") && !equal(ruleGet(rules, "const")) ||
		ruleHas(rules, "in") && !slices.ContainsFunc(ruleList(rules, "in"), equal) ||
		slices.ContainsFunc(ruleList(rules, "not_in"), equal) {
		return false
	}
	within := func(b numberBound, beyond int) bool {
		if b.name == "" {
			return true
		}
		c, ok := compare(ruleGet(rules, b.name))
		return ok && (c == beyond || c == 0 && !b.exclusive)
	}
	low, high := bound(rules, "gt", "gte", true), bound(rules, "lt", "lte", false)
	if inverted(low, high) {
		return within(low, 1) || within(high, -1)
	}
	return within(low, 1) && within(high, -1)
}

func floatAccepts(rules protoreflect.Message, x float64) bool {
	if ruleBool(rules, "finite") && (math.IsNaN(x) || math.IsInf(x, 0)) {
		return false
	}
	return numberAccepts(rules, func(v protoreflect.Value) (int, bool) {
		r := v.Float()
		return cmp.Compare(x, r), !math.IsNaN(x) && !math.IsNaN(r)
	})
}

func intAccepts(rules protoreflect.Message, x *big.Float) bool {
	return numberAccepts(rules, func(v protoreflect.Value) (int, bool) {
		return x.Cmp(intValue(v)), true
	})
}

// floatSpecials returns the strings ProtoJSON writes for NaN and the
// infinities that the rules accept.
func floatSpecials(rules protoreflect.Message) []any {
	var out []any
	for _, special := range []string{"NaN", "Infinity", "-Infinity"} {
		if f, _ := strconv.ParseFloat(special, 64); floatAccepts(rules, f) {
			out = append(out, special)
		}
	}
	return out
}

// never is a schema no value satisfies.
func never() *jsonschema.Schema {
	return &jsonschema.Schema{Not: &jsonschema.Schema{}}
}

// appendUnique appends v unless list has it: draft-04/06/07 require unique
// enum items. Custom enum values may be objects or arrays.
func appendUnique(list []any, v any) []any {
	if containsValue(list, v) {
		return list
	}
	return append(list, v)
}

func containsValue(list []any, v any) bool {
	return slices.ContainsFunc(list, func(o any) bool { return reflect.DeepEqual(o, v) })
}

// restrict applies const and in, which both hold: the const if in has it,
// else nothing.
func restrict(schema *jsonschema.Schema, c any, hasConst bool, in []any, hasIn bool) {
	switch {
	case hasConst && (!hasIn || containsValue(in, c)):
		schema.Const = &c
	case hasIn && !hasConst && len(in) > 0:
		schema.Enum = in
	case hasConst || hasIn:
		schema.AllOf = append(schema.AllOf, never())
	}
}

// acceptsEmptyString reports whether string rules accept "", the value of
// an absent field, including patterns the schema cannot express. Of the
// formats, only a URI reference may be empty.
func acceptsEmptyString(rules protoreflect.Message) bool {
	empty := func(v protoreflect.Value) bool { return v.String() == "" }
	for _, name := range []protoreflect.Name{"const", "prefix", "suffix", "contains"} {
		if ruleHas(rules, name) && !empty(ruleGet(rules, name)) {
			return false
		}
	}
	for _, name := range []protoreflect.Name{"email", "hostname", "ip", "ipv4", "ipv6", "uri", "uuid"} {
		if ruleBool(rules, name) {
			return false
		}
	}
	if ruleUint(rules, "len") != 0 || ruleUint(rules, "min_len") != 0 ||
		ruleHas(rules, "not_contains") && empty(ruleGet(rules, "not_contains")) ||
		ruleHas(rules, "in") && !slices.ContainsFunc(ruleList(rules, "in"), empty) ||
		slices.ContainsFunc(ruleList(rules, "not_in"), empty) {
		return false
	}
	if ruleHas(rules, "pattern") {
		if re, err := regexp.Compile(ruleGet(rules, "pattern").String()); err == nil && !re.MatchString("") {
			return false
		}
	}
	return true
}

// stringFormats maps string rules to JSON Schema formats; at most one is set.
// Rules that protovalidate defines exactly become patterns instead.
var stringFormats = []struct {
	rule   protoreflect.Name
	format string
}{
	{"uri", "uri"},
	{"uri_ref", "uri-reference"},
}

func applyStringRules(schema *jsonschema.Schema, rules protoreflect.Message) {
	if rules == nil {
		return
	}
	str := func(name protoreflect.Name) string { return ruleGet(rules, name).String() }
	strs := func(name protoreflect.Name) []any {
		var out []any
		for _, v := range ruleList(rules, name) {
			out = appendUnique(out, v.String())
		}
		return out
	}

	// len, min_len and max_len all apply.
	if ruleHas(rules, "len") {
		schema.MinLength, schema.MaxLength = ruleSize(rules, "len"), ruleSize(rules, "len")
	}
	if n := ruleSize(rules, "min_len"); ruleHas(rules, "min_len") && (schema.MinLength == nil || *n > *schema.MinLength) {
		schema.MinLength = n
	}
	if n := ruleSize(rules, "max_len"); ruleHas(rules, "max_len") && (schema.MaxLength == nil || *n < *schema.MaxLength) {
		schema.MaxLength = n
	}

	restrict(schema, str("const"), ruleHas(rules, "const"), strs("in"), ruleHas(rules, "in"))
	if notIn := strs("not_in"); len(notIn) > 0 {
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Not: &jsonschema.Schema{Enum: notIn}})
	}

	var patterns []string
	if ruleHas(rules, "pattern") {
		if pattern, ok := ecmaPattern(str("pattern")); ok {
			patterns = append(patterns, pattern)
		}
	}
	if ruleHas(rules, "prefix") {
		patterns = append(patterns, "^"+regexp.QuoteMeta(str("prefix")))
	}
	if ruleHas(rules, "suffix") {
		patterns = append(patterns, regexp.QuoteMeta(str("suffix"))+"$")
	}
	if ruleHas(rules, "contains") {
		patterns = append(patterns, regexp.QuoteMeta(str("contains")))
	}
	if ruleBool(rules, "email") {
		patterns = append(patterns, emailPattern)
	}
	if ruleBool(rules, "uuid") {
		patterns = append(patterns, uuidPattern)
	}
	if ruleBool(rules, "ipv4") {
		patterns = append(patterns, ipv4Pattern)
	}
	if ruleBool(rules, "hostname") {
		// The 253 characters exclude a trailing dot.
		patterns = append(patterns, hostnamePattern, hostnameLastLabelPattern)
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{AnyOf: []*jsonschema.Schema{
			{MaxLength: new(253)},
			{MaxLength: new(254), Pattern: `\.$`},
		}})
	}
	for _, pattern := range patterns {
		if schema.Pattern == "" {
			schema.Pattern = pattern
		} else {
			schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Pattern: pattern})
		}
	}
	if ruleHas(rules, "not_contains") {
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Not: &jsonschema.Schema{Pattern: regexp.QuoteMeta(str("not_contains"))}})
	}

	for _, f := range stringFormats {
		if ruleBool(rules, f.rule) {
			schema.Format = f.format
		}
	}
	// protovalidate also accepts an IPv6 address with a zone.
	ipv6 := []*jsonschema.Schema{{Format: "ipv6"}, {Pattern: zonedIPv6Pattern}}
	switch {
	case ruleBool(rules, "ip"):
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{AnyOf: append([]*jsonschema.Schema{{Pattern: ipv4Pattern}}, ipv6...)})
	case ruleBool(rules, "ipv6"):
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{AnyOf: ipv6})
	}
}

// zonedIPv6Pattern is an IPv6 address (RFC 4291) with a zone, which
// protovalidate takes as any nonempty string after "%", and not after an
// address ending in IPv4 notation.
const zonedIPv6Pattern = "^(?:(?:[0-9A-Fa-f]{1,4}:){6}[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|::(?:[0-9A-Fa-f]{1,4}:){5}[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|(?:[0-9A-Fa-f]{1,4})?::(?:[0-9A-Fa-f]{1,4}:){4}[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|(?:(?:[0-9A-Fa-f]{1,4}:){0,1}[0-9A-Fa-f]{1,4})?::(?:[0-9A-Fa-f]{1,4}:){3}[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|(?:(?:[0-9A-Fa-f]{1,4}:){0,2}[0-9A-Fa-f]{1,4})?::(?:[0-9A-Fa-f]{1,4}:){2}[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|(?:(?:[0-9A-Fa-f]{1,4}:){0,3}[0-9A-Fa-f]{1,4})?::[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|(?:(?:[0-9A-Fa-f]{1,4}:){0,4}[0-9A-Fa-f]{1,4})?::[0-9A-Fa-f]{1,4}:[0-9A-Fa-f]{1,4}|(?:(?:[0-9A-Fa-f]{1,4}:){0,5}[0-9A-Fa-f]{1,4})?::[0-9A-Fa-f]{1,4}|(?:(?:[0-9A-Fa-f]{1,4}:){0,6}[0-9A-Fa-f]{1,4})?::)%[\\s\\S]+$"

// ipv4Pattern is protovalidate's IPv4 address: four decimal octets up to
// 255, without leading zeros.
const ipv4Pattern = `^(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])(?:\.(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])){3}$`

// uuidPattern is protovalidate's uuid rule, which checks no version.
const uuidPattern = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`

// hostnamePattern is protovalidate's hostname: dot-separated labels of 1 to
// 63 letters, digits and inner hyphens, and an optional trailing dot. Format
// hostname (RFC 1123) has no trailing dot and allows a numeric last label,
// which hostnameLastLabelPattern rules out.
const (
	hostnamePattern          = `^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*\.?$`
	hostnameLastLabelPattern = `[^0-9.][^.]*\.?$`
)

// emailPattern is protovalidate's email rule: the HTML standard's valid
// email address, which format "email" (RFC 5321) does not match.
const emailPattern = "^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"

// ecmaPattern translates an RE2 pattern, as protovalidate uses, to the
// ECMA-262 syntax of JSON Schema, from its parse tree: the two differ in
// meaning as well as syntax, as for "." and "\\x{...}". ok is false for
// what has no translation, such as multi-line mode.
func ecmaPattern(re2 string) (string, bool) {
	re, err := syntax.Parse(re2, syntax.Perl)
	if err != nil {
		return "", false
	}
	var b strings.Builder
	if !writeECMA(&b, re) {
		return "", false
	}
	return b.String(), true
}

func writeECMA(b *strings.Builder, re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpNoMatch:
		b.WriteString(`[^\s\S]`)
	case syntax.OpEmptyMatch:
		b.WriteString("(?:)")
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			if re.Flags&syntax.FoldCase != 0 && unicode.SimpleFold(r) != r {
				// (?i) has no ECMA-262 form: list the cases.
				folds := []rune{r}
				for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
					folds = append(folds, f)
				}
				slices.Sort(folds)
				var ranges []rune
				for _, f := range folds {
					ranges = append(ranges, f, f)
				}
				if !writeClass(b, ranges) {
					return false
				}
				continue
			}
			writeRune(b, r, false)
		}
	case syntax.OpCharClass:
		return writeClass(b, re.Rune)
	case syntax.OpAnyCharNotNL:
		// RE2 "." matches \r, ECMA-262 "." does not.
		b.WriteString(`[^\n]`)
	case syntax.OpAnyChar:
		b.WriteString(`[\s\S]`)
	case syntax.OpBeginText:
		b.WriteString("^")
	case syntax.OpEndText:
		b.WriteString("$")
	case syntax.OpWordBoundary:
		b.WriteString(`\b`)
	case syntax.OpNoWordBoundary:
		b.WriteString(`\B`)
	case syntax.OpCapture:
		// Without the name: RE2 allows names that ECMA-262 does not, such as
		// "1" or a repeated one, and a pattern only matches.
		b.WriteString("(?:")
		if !writeECMA(b, re.Sub[0]) {
			return false
		}
		b.WriteString(")")
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		b.WriteString("(?:")
		if !writeECMA(b, re.Sub[0]) {
			return false
		}
		b.WriteString(")")
		switch re.Op {
		case syntax.OpStar:
			b.WriteString("*")
		case syntax.OpPlus:
			b.WriteString("+")
		case syntax.OpQuest:
			b.WriteString("?")
		default:
			b.WriteString("{" + strconv.Itoa(re.Min))
			if re.Max != re.Min {
				b.WriteString(",")
				if re.Max >= 0 {
					b.WriteString(strconv.Itoa(re.Max))
				}
			}
			b.WriteString("}")
		}
		if re.Flags&syntax.NonGreedy != 0 {
			b.WriteString("?")
		}
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			if !writeECMA(b, sub) {
				return false
			}
		}
	case syntax.OpAlternate:
		b.WriteString("(?:")
		for i, sub := range re.Sub {
			if i > 0 {
				b.WriteString("|")
			}
			if !writeECMA(b, sub) {
				return false
			}
		}
		b.WriteString(")")
	default:
		// ^ and $ in multi-line mode.
		return false
	}
	return true
}

// writeClass writes a class of rune ranges. Beyond the BMP, a class means
// code points only in ECMA-262's unicode mode, which JSON Schema leaves open.
func writeClass(b *strings.Builder, ranges []rune) bool {
	b.WriteString("[")
	for i := 0; i < len(ranges); i += 2 {
		lo, hi := ranges[i], ranges[i+1]
		if hi > 0xFFFF {
			return false
		}
		writeRune(b, lo, true)
		if hi != lo {
			b.WriteString("-")
			writeRune(b, hi, true)
		}
	}
	b.WriteString("]")
	return true
}

// writeRune writes r as itself, escaped where it has a meaning; control
// characters as \xHH, which RE2 and ECMA-262 both read.
func writeRune(b *strings.Builder, r rune, inClass bool) {
	switch {
	case r < 0x20 || r == 0x7F:
		fmt.Fprintf(b, `\x%02X`, r)
	case inClass && strings.ContainsRune(`\]-^[`, r), !inClass && strings.ContainsRune(`\^$.|?*+()[]{}/`, r):
		b.WriteRune('\\')
		b.WriteRune(r)
	default:
		b.WriteRune(r)
	}
}

// enumNumbers returns the numbers that const, in and defined_only allow, in
// the order of the rules then of the enum, and whether those rules are set.
func enumNumbers(enum protoreflect.EnumDescriptor, rules protoreflect.Message) ([]protoreflect.EnumNumber, bool) {
	if !ruleHas(rules, "const") && !ruleHas(rules, "in") && !ruleBool(rules, "defined_only") {
		return nil, false
	}
	var candidates []protoreflect.EnumNumber
	if ruleHas(rules, "const") {
		candidates = append(candidates, protoreflect.EnumNumber(ruleGet(rules, "const").Int()))
	}
	for _, v := range ruleList(rules, "in") {
		candidates = append(candidates, protoreflect.EnumNumber(v.Int()))
	}
	for value := range listOf(enum.Values()) {
		candidates = append(candidates, value.Number())
	}
	var allowed []protoreflect.EnumNumber
	for _, n := range candidates {
		if enumAccepts(rules, enum, n) && !slices.Contains(allowed, n) {
			allowed = append(allowed, n)
		}
	}
	return allowed, true
}

// applyEnumRules restricts schema, a reference to enum, to the JSON forms
// of the values the rules allow.
func applyEnumRules(pluginOptions *proto.PluginOptions, schema *jsonschema.Schema, enum protoreflect.EnumDescriptor, rules protoreflect.Message) {
	forms := func(numbers []protoreflect.EnumNumber) []any {
		var out []any
		for _, n := range numbers {
			for _, v := range enumJSONValues(pluginOptions, enum, n) {
				out = appendUnique(out, v)
			}
		}
		return out
	}
	// Canonical writes defined values only, as the reference already says.
	allowed, restricted := enumNumbers(enum, rules)
	if restricted && (isLenient(pluginOptions) || ruleHas(rules, "const") || ruleHas(rules, "in")) {
		if values := forms(allowed); len(values) > 0 {
			schema.Enum = values
		} else {
			schema.AllOf = append(schema.AllOf, never())
		}
		return
	}
	// Distinct numbers may share a custom value: exclude only values that no
	// allowed number has.
	var excluded, kept []protoreflect.EnumNumber
	for value := range listOf(enum.Values()) {
		if enumAccepts(rules, enum, value.Number()) {
			kept = append(kept, value.Number())
		}
	}
	for _, v := range ruleList(rules, "not_in") {
		excluded = append(excluded, protoreflect.EnumNumber(v.Int()))
	}
	keptForms := forms(kept)
	values := slices.DeleteFunc(forms(excluded), func(v any) bool { return containsValue(keptForms, v) })
	if len(values) > 0 {
		schema.AllOf = append(schema.AllOf, &jsonschema.Schema{Not: &jsonschema.Schema{Enum: values}})
	}
}

// enumJSONValues returns the JSON forms of the enum value number that
// buildFromEnum lists: every name with that number, the number in Lenient.
func enumJSONValues(pluginOptions *proto.PluginOptions, enum protoreflect.EnumDescriptor, number protoreflect.EnumNumber) []any {
	eo := proto.GetEnumOptions(enum)
	lenient := isLenient(pluginOptions) && eo.GetMappingType() != proto.EnumOptions_MapToCustom
	switch {
	case !lenient && eo.GetMappingType() == proto.EnumOptions_MapToNumber:
		return []any{int32(number)}
	case !lenient && eo.GetMappingType() == proto.EnumOptions_MapToCustom:
		var out []any
		for v := range listOf(enum.Values()) {
			if v.Number() != number {
				continue
			}
			if custom, err := parseScalaValueFromAny(proto.GetEnumValueOptions(v).GetCustomValue()); err == nil && custom != nil {
				out = appendUnique(out, custom)
			} else {
				out = appendUnique(out, string(v.Name()))
			}
		}
		return out
	}
	var out []any
	for v := range listOf(enum.Values()) {
		if v.Number() == number {
			out = append(out, string(v.Name()))
		}
	}
	if lenient {
		out = append(out, int32(number))
	}
	return out
}

// enumNumberRules is a schema of the enum numbers the rules allow.
func enumNumberRules(enum protoreflect.EnumDescriptor, rules protoreflect.Message) *jsonschema.Schema {
	numbers := func(list []protoreflect.EnumNumber) []any {
		var out []any
		for _, n := range list {
			out = appendUnique(out, int32(n))
		}
		return out
	}
	if allowed, restricted := enumNumbers(enum, rules); restricted {
		if len(allowed) == 0 {
			return never()
		}
		return &jsonschema.Schema{Enum: numbers(allowed)}
	}
	var excluded []protoreflect.EnumNumber
	for _, v := range ruleList(rules, "not_in") {
		excluded = append(excluded, protoreflect.EnumNumber(v.Int()))
	}
	if len(excluded) == 0 {
		return &jsonschema.Schema{}
	}
	return &jsonschema.Schema{Not: &jsonschema.Schema{Enum: numbers(excluded)}}
}

// enumAccepts reports whether enum rules accept number.
func enumAccepts(rules protoreflect.Message, enum protoreflect.EnumDescriptor, number protoreflect.EnumNumber) bool {
	is := func(v protoreflect.Value) bool { return protoreflect.EnumNumber(v.Int()) == number }
	return (!ruleHas(rules, "const") || is(ruleGet(rules, "const"))) &&
		(!ruleHas(rules, "in") || slices.ContainsFunc(ruleList(rules, "in"), is)) &&
		!slices.ContainsFunc(ruleList(rules, "not_in"), is) &&
		(!ruleBool(rules, "defined_only") || enum.Values().ByNumber(number) != nil)
}
