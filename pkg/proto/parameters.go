package proto

import (
	"strconv"
	"strings"
)

// Parameters are the plugin parameters protoc passes as
// "name=value,name2=value2" (from --jsonschema_opt and --jsonschema_out).
type Parameters map[string]string

func ParseParameters(s string) Parameters {
	params := Parameters{}
	if s == "" {
		return params
	}
	for part := range strings.SplitSeq(s, ",") {
		name, value, _ := strings.Cut(part, "=")
		params[name] = value
	}
	return params
}

func (p Parameters) Str(name string) string { return p[name] }

func (p Parameters) StrDefault(name, def string) string {
	if v, ok := p[name]; ok {
		return v
	}
	return def
}

// BoolDefault treats a name given without a value ("pretty_json_output") as true.
func (p Parameters) BoolDefault(name string, def bool) (bool, error) {
	v, ok := p[name]
	if !ok {
		return def, nil
	}
	if strings.TrimSpace(v) == "" {
		return true, nil
	}
	return strconv.ParseBool(v)
}

func (p Parameters) UintDefault(name string, def uint) (uint, error) {
	v, ok := p[name]
	if !ok {
		return def, nil
	}
	n, err := strconv.ParseUint(v, 10, strconv.IntSize)
	return uint(n), err
}
