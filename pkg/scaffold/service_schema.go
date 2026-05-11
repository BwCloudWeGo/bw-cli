package scaffold

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	relationHasOne    = "has_one"
	relationHasMany   = "has_many"
	relationBelongsTo = "belongs_to"
)

// ServiceSchema describes table-driven service generation options loaded from YAML.
type ServiceSchema struct {
	Table          string           `yaml:"table"`
	Resource       string           `yaml:"resource"`
	ExcludeFields  []string         `yaml:"exclude_fields"`
	ReadonlyFields []string         `yaml:"readonly_fields"`
	Relations      []RelationSchema `yaml:"relations"`
}

// RelationSchema describes one associated relational table.
type RelationSchema struct {
	Name         string   `yaml:"name"`
	Table        string   `yaml:"table"`
	Type         string   `yaml:"type"`
	LocalField   string   `yaml:"local_field"`
	ForeignField string   `yaml:"foreign_field"`
	Methods      []string `yaml:"methods"`
}

func loadServiceSchema(path string) (ServiceSchema, error) {
	if strings.TrimSpace(path) == "" {
		return ServiceSchema{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ServiceSchema{}, err
	}
	var schema ServiceSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return ServiceSchema{}, fmt.Errorf("parse service schema %s: %w", path, err)
	}
	return schema, nil
}

func mergeServiceSchema(schema ServiceSchema, tableOverride string, defaultResource string) (ServiceSchema, error) {
	schema.Table = firstNonEmpty(tableOverride, schema.Table)
	schema.Resource = firstNonEmpty(schema.Resource, defaultResource)
	schema.Table = strings.TrimSpace(schema.Table)
	schema.Resource = strings.TrimSpace(schema.Resource)
	schema.ExcludeFields = cleanStringList(schema.ExcludeFields)
	schema.ReadonlyFields = cleanStringList(schema.ReadonlyFields)
	for i := range schema.Relations {
		relation := &schema.Relations[i]
		relation.Name = strings.TrimSpace(relation.Name)
		relation.Table = strings.TrimSpace(relation.Table)
		relation.Type = strings.TrimSpace(relation.Type)
		relation.LocalField = strings.TrimSpace(relation.LocalField)
		relation.ForeignField = strings.TrimSpace(relation.ForeignField)
		relation.Methods = cleanStringList(relation.Methods)
		if relation.Name == "" {
			relation.Name = relation.Table
		}
		if err := validateRelationSchema(*relation); err != nil {
			return ServiceSchema{}, err
		}
	}
	return schema, nil
}

func validateRelationSchema(relation RelationSchema) error {
	if relation.Table == "" {
		return fmt.Errorf("relation %q table is required", relation.Name)
	}
	switch relation.Type {
	case relationHasOne, relationHasMany, relationBelongsTo:
	default:
		return fmt.Errorf("relation %q has unsupported relation type %q", relation.Name, relation.Type)
	}
	if relation.LocalField == "" {
		return fmt.Errorf("relation %q local_field is required", relation.Name)
	}
	if relation.ForeignField == "" {
		return fmt.Errorf("relation %q foreign_field is required", relation.Name)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cleanStringList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		cleaned = append(cleaned, value)
	}
	return cleaned
}
