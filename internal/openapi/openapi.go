package openapi

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ErrUnsupportedVersion is returned for Swagger 2.0 (or otherwise unsupported) docs.
var ErrUnsupportedVersion = errors.New("unsupported OpenAPI version (only 3.x)")

// Operation is a normalized HTTP operation.
type Operation struct {
	Method         string
	Path           string
	OperationID    string
	Summary        string
	RequestSchema  string
	ResponseSchema string
	Security       []string
	Tags           []string
}

// Schema is a normalized component schema.
type Schema struct {
	Name   string
	RawRef string
}

// Spec is a normalized OpenAPI document.
type Spec struct {
	Name       string
	Version    string
	Operations []Operation
	Schemas    []Schema
}

// ParseFile parses an OpenAPI 3.x document at path.
func ParseFile(path string) (*Spec, error) {
	// #nosec G304 -- OpenAPI paths are explicit user manifest inputs.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Reject Swagger 2.0 before handing to the 3.x loader.
	if strings.Contains(string(data), `swagger: "2.0"`) || strings.Contains(string(data), `"swagger":"2.0"`) {
		return nil, ErrUnsupportedVersion
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, err
	}
	return normalize(doc), nil
}

func refName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func normalize(doc *openapi3.T) *Spec {
	s := &Spec{}
	if doc.Info != nil {
		s.Name = doc.Info.Title
		s.Version = doc.Info.Version
	}
	for _, path := range sortedKeys(doc.Paths.Map()) {
		item := doc.Paths.Value(path)
		for method, op := range item.Operations() {
			no := Operation{
				Method:      method,
				Path:        path,
				OperationID: op.OperationID,
				Summary:     op.Summary,
				Tags:        op.Tags,
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				if mt := op.RequestBody.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Ref != "" {
					no.RequestSchema = refName(mt.Schema.Ref)
				}
			}
			for _, code := range sortedRespCodes(op.Responses) {
				resp := op.Responses.Value(code)
				if resp.Value == nil {
					continue
				}
				if mt := resp.Value.Content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Ref != "" {
					no.ResponseSchema = refName(mt.Schema.Ref)
					break
				}
			}
			no.Security = append(no.Security, securityNames(op.Security)...)
			s.Operations = append(s.Operations, no)
		}
	}
	if doc.Components != nil {
		for _, name := range sortedKeys(mapToSet(doc.Components.Schemas)) {
			s.Schemas = append(s.Schemas, Schema{Name: name, RawRef: "#/components/schemas/" + name})
		}
	}
	return s
}

func securityNames(reqs *openapi3.SecurityRequirements) []string {
	var out []string
	if reqs == nil {
		return out
	}
	for _, req := range *reqs {
		for name := range req {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mapToSet(m openapi3.Schemas) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

func sortedRespCodes(r *openapi3.Responses) []string {
	if r == nil {
		return nil
	}
	return sortedKeys(r.Map())
}
