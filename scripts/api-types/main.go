package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	sim "traffic/internal/simulation"
	"unicode"
)

var declarations = map[string]string{}

func name(t reflect.Type) string {
	if t == reflect.TypeOf(sim.Snapshot{}) {
		return "Snapshot"
	}
	if t == reflect.TypeOf(sim.CityView{}) {
		return "City"
	}
	p := t.PkgPath()
	p = p[strings.LastIndex(p, "/")+1:]
	parts := strings.Split(p, "_")
	for i, s := range parts {
		parts[i] = strings.ToUpper(s[:1]) + s[1:]
	}
	return strings.Join(parts, "") + t.Name()
}
func typ(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return typ(t.Elem())
	case reflect.Slice, reflect.Array:
		return "Array<" + typ(t.Elem()) + ">"
	case reflect.Map:
		return "Record<string, " + typ(t.Elem()) + ">"
	case reflect.Bool:
		return "boolean"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Struct:
		n := name(t)
		if _, ok := declarations[n]; ok {
			return n
		}
		declarations[n] = ""
		fields := []string{}
		parents := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !unicode.IsUpper([]rune(f.Name)[0]) {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "-" {
				continue
			}
			if f.Anonymous && tag[0] == "" {
				parents = append(parents, typ(f.Type))
				continue
			}
			key := tag[0]
			if key == "" {
				key = f.Name
			}
			optional := ""
			if strings.Contains(f.Tag.Get("json"), "omitempty") {
				optional = "?"
			}
			fields = append(fields, fmt.Sprintf("  %q%s: %s;", key, optional, typ(f.Type)))
		}
		ext := ""
		if len(parents) > 0 {
			ext = " extends " + strings.Join(parents, ", ")
		}
		declarations[n] = "export interface " + n + ext + " {\n" + strings.Join(fields, "\n") + "\n}\n"
		return n
	}
	return "unknown"
}
func main() {
	typ(reflect.TypeOf(sim.Snapshot{}))
	typ(reflect.TypeOf(sim.CityView{}))
	keys := []string{}
	for k := range declarations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := "// Generated from Go JSON contracts by go run ./scripts/api-types. Do not hand edit.\n"
	for _, k := range keys {
		s += declarations[k] + "\n"
	}
	if e := os.WriteFile("frontend/src/api/models.ts", []byte(s), 0644); e != nil {
		panic(e)
	}
}
