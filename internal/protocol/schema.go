package protocol

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/theclifmeister/kingpin/internal/engine"
	"github.com/theclifmeister/kingpin/internal/events"
)

// Schema is the protocol as a JSON Schema document (draft 2020-12): every
// method with its positional parameters and its result, the two
// notifications, and a definition for every named type they carry,
// every event kind's payload among them. It is generated from the Go
// types and checked in as schema.json, so a client in another language
// can generate its bindings from it; TestSchemaIsCurrent fails when the
// file is stale.
func Schema() ([]byte, error) {
	g := &schemaGen{defs: map[string]any{}}
	ms := map[string]any{}
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		m := methods[name]
		ps := []any{}
		for _, p := range m.params {
			s := g.of(p.typ)
			if names := dialNames(p.typ); names != nil {
				s = map[string]any{"type": "string", "enum": names} // a dial goes in by name
			}
			s["title"] = p.name
			ps = append(ps, s)
		}
		entry := map[string]any{
			"params":   map[string]any{"type": "array", "prefixItems": ps, "minItems": len(ps), "maxItems": len(ps)},
			"needsRun": m.run,
			"changes":  m.changes,
		}
		if m.result != nil {
			entry["result"] = g.of(m.result)
		} else {
			entry["result"] = map[string]any{"type": "null"}
		}
		ms[name] = entry
	}
	kinds := map[string]any{}
	for _, e := range events.All {
		kinds[e.Kind()] = g.of(reflect.TypeOf(e))
	}
	doc := map[string]any{
		"$schema":      "https://json-schema.org/draft/2020-12/schema",
		"title":        "kingpind",
		"protocol":     Version,
		"view_version": engine.ViewVersion,
		"framing":      "JSON-RPC 2.0, one message a line; params by position; notifications before the response they belong to",
		"errors": map[string]int{
			"parse": CodeParse, "invalid_request": CodeInvalidRequest, "no_method": CodeNoMethod,
			"invalid_params": CodeInvalidParams, "internal": CodeInternal, "refused": CodeRefused, "no_run": CodeNoRun,
		},
		"methods": ms,
		"notifications": map[string]any{
			"event": map[string]any{"type": "object", "properties": map[string]any{
				"kind":    map[string]any{"type": "string", "enum": sortedKinds()},
				"day":     map[string]any{"type": "integer"},
				"payload": map[string]any{"description": "the event, its schema under events by kind"},
				"cue":     g.of(reflect.TypeFor[engine.Cue]()),
			}, "required": []string{"kind", "day", "payload"}},
			"view": g.of(reflect.TypeFor[engine.View]()),
		},
		"events": kinds,
		"$defs":  g.defs,
	}
	return json.MarshalIndent(doc, "", "  ")
}

func sortedKinds() []string {
	var out []string
	for _, e := range events.All {
		out = append(out, e.Kind())
	}
	sort.Strings(out)
	return out
}

type schemaGen struct{ defs map[string]any }

// of is t's schema as encoding/json writes it: a named struct by
// reference to its definition, a dial as its int (inside a result or an
// event a dial is the int a save holds; x-names names them in order).
func (g *schemaGen) of(t reflect.Type) map[string]any {
	if names := dialNames(t); names != nil {
		return map[string]any{"type": "integer", "minimum": 0, "maximum": len(names) - 1, "x-names": names}
	}
	if t == termsType {
		t = reflect.TypeFor[TermsParams]()
	}
	switch t.Kind() {
	case reflect.Pointer:
		return g.of(t.Elem())
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": g.of(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": g.of(t.Elem())}
	case reflect.Interface:
		return map[string]any{}
	case reflect.Struct:
		name := t.Name()
		if name == "" {
			return g.object(t)
		}
		if pkg := t.PkgPath(); pkg != "" {
			name = pkg[strings.LastIndex(pkg, "/")+1:] + "." + name
		}
		if _, ok := g.defs[name]; !ok {
			g.defs[name] = map[string]any{} // a placeholder: a type that holds itself stops here
			g.defs[name] = g.object(t)
		}
		return map[string]any{"$ref": "#/$defs/" + name}
	}
	return map[string]any{}
}

// object is a struct's properties as encoding/json writes them.
func (g *schemaGen) object(t reflect.Type) map[string]any {
	props := map[string]any{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		props[name] = g.of(f.Type)
		if !strings.Contains(opts, "omitempty") {
			required = append(required, name)
		}
	}
	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}
