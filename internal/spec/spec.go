// Package spec reads the description an agent serves of its own API. Without it the CLI can only
// pass fields through and hope.
package spec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Spec struct {
	OpenAPI    string                          `json:"openapi"`
	Info       Info                            `json:"info"`
	Paths      map[string]map[string]Operation `json:"paths"`
	Components Components                      `json:"components"`
}

type Info struct {
	Version string `json:"version"`
}

type Components struct {
	Schemas map[string]*Schema `json:"schemas"`
}

type Operation struct {
	OperationID string       `json:"operationId"`
	Parameters  []Parameter  `json:"parameters"`
	RequestBody *RequestBody `json:"requestBody"`
	Responses   map[string]struct {
		Content map[string]struct {
			Schema *Schema `json:"schema"`
		} `json:"content"`
	} `json:"responses"`
	Permission string `json:"x-permission"`
}

type Parameter struct {
	Name     string  `json:"name"`
	In       string  `json:"in"`
	Required bool    `json:"required"`
	Schema   *Schema `json:"schema"`
}

type RequestBody struct {
	Required bool `json:"required"`
	Content  map[string]struct {
		Schema *Schema `json:"schema"`
	} `json:"content"`
}

type Schema struct {
	Ref                  string             `json:"$ref"`
	Type                 string             `json:"type"`
	Format               string             `json:"format"`
	Items                *Schema            `json:"items"`
	Properties           map[string]*Schema `json:"properties"`
	PropertyOrder        []string           `json:"x-property-order"`
	Columns              []string           `json:"x-columns"`
	Render               string             `json:"x-render"`
	Required             []string           `json:"required"`
	Description          string             `json:"description"`
	AdditionalProperties *Schema            `json:"additionalProperties"`
}

func Parse(raw []byte) (*Spec, error) {
	var s Spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("the agent's API description could not be read: %w", err)
	}
	if len(s.Paths) == 0 {
		return nil, fmt.Errorf("the agent's API description contains no endpoints")
	}
	return &s, nil
}

// Resolve follows a $ref to the schema it names.
func (s *Spec) Resolve(schema *Schema) *Schema {
	seen := 0
	for schema != nil && schema.Ref != "" && seen < 10 {
		name := strings.TrimPrefix(schema.Ref, "#/components/schemas/")
		schema = s.Components.Schemas[name]
		seen++
	}
	return schema
}

// Operation takes the path as the CLI holds it, `/deployments/:name`, not as the spec writes it.
func (s *Spec) Operation(method, path string) (Operation, bool) {
	op, ok := s.Paths[specPath(path)][strings.ToLower(method)]
	return op, ok
}

func specPath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			segments[i] = "{" + strings.TrimPrefix(segment, ":") + "}"
		}
	}
	return "/api" + strings.Join(segments, "/")
}

// Field is one thing an endpoint accepts in its body.
type Field struct {
	Name     string
	Type     string
	Required bool
	Help     string
}

// Fields are an endpoint's body fields in declaration order, so help reads the way the type does.
func (s *Spec) Fields(op Operation) []Field {
	if op.RequestBody == nil {
		return nil
	}
	content, ok := op.RequestBody.Content["application/json"]
	if !ok {
		return nil
	}
	schema := s.Resolve(content.Schema)
	if schema == nil || len(schema.Properties) == 0 {
		return nil
	}

	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}

	order := schema.PropertyOrder
	if len(order) == 0 {
		for name := range schema.Properties {
			order = append(order, name)
		}
		sort.Strings(order)
	}

	fields := make([]Field, 0, len(order))
	for _, name := range order {
		property := s.Resolve(schema.Properties[name])
		if property == nil {
			continue
		}
		fields = append(fields, Field{
			Name:     name,
			Type:     typeName(property),
			Required: required[name],
			Help:     property.Description,
		})
	}
	return fields
}

func typeName(schema *Schema) string {
	switch schema.Type {
	case "array":
		if schema.Items != nil && schema.Items.Type != "" {
			return schema.Items.Type + " list"
		}
		return "list"
	case "":
		return "any"
	case "string":
		if schema.Format == "date-time" {
			return "timestamp"
		}
		return "string"
	}
	return schema.Type
}

// QueryParams are the query keys an endpoint reads.
func (s *Spec) QueryParams(op Operation) []string {
	var names []string
	for _, p := range op.Parameters {
		if p.In == "query" {
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
}

// Shape is how an endpoint's answer is presented: "list", "item", "message", or empty when the
// agent does not say. Key holds the rows or the thing within the answer.
type Shape struct {
	Kind    string
	Key     string
	Columns []string
}

// maxColumns keeps a wide type readable. Everything is still in --json.
const maxColumns = 6

func (s *Spec) Shape(op Operation) (Shape, bool) {
	ok200, ok := op.Responses["200"]
	if !ok {
		return Shape{}, false
	}
	content, ok := ok200.Content["application/json"]
	if !ok {
		return Shape{}, false
	}
	schema := s.Resolve(content.Schema)
	if schema == nil || schema.Render == "" {
		return Shape{}, false
	}

	shape := Shape{Kind: schema.Render}
	for _, name := range propertyOrder(schema) {
		property := schema.Properties[name]
		if property == nil {
			continue
		}
		switch schema.Render {
		case "list":
			if property.Type != "array" || property.Items == nil {
				continue
			}
			shape.Key = name
			if row := s.Resolve(property.Items); row != nil {
				shape.Columns = row.Columns
				if len(shape.Columns) > maxColumns {
					shape.Columns = shape.Columns[:maxColumns]
				}
			}
			return shape, true
		case "item":
			shape.Key = name
			return shape, true
		}
	}
	if schema.Render == "message" {
		return shape, true
	}
	return Shape{}, false
}

func propertyOrder(schema *Schema) []string {
	if len(schema.PropertyOrder) > 0 {
		return schema.PropertyOrder
	}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
