package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RelationshipType 表示两张表在生成计划中的关联方向。
type RelationshipType string

const (
	RelationshipOneToOne   RelationshipType = "one_to_one"
	RelationshipOneToMany  RelationshipType = "one_to_many"
	RelationshipManyToOne  RelationshipType = "many_to_one"
	RelationshipManyToMany RelationshipType = "many_to_many"
)

// TableRelationship 描述生成服务时需要额外加载的表关系。
type TableRelationship struct {
	Type       RelationshipType `json:"type"`
	FromTable  string           `json:"from_table"`
	FromColumn string           `json:"from_column"`
	ToTable    string           `json:"to_table"`
	ToColumn   string           `json:"to_column"`
	JoinTable  string           `json:"join_table,omitempty"`
}

// GenerationPlan 是 designer 和 CLI 共用的代码生成计划。
type GenerationPlan struct {
	ServiceName   string              `json:"service_name"`
	RootTable     string              `json:"root_table"`
	Schema        string              `json:"schema,omitempty"`
	Tables        []string            `json:"tables"`
	Relationships []TableRelationship `json:"relationships"`
}

// LoadGenerationPlan 从 JSON 文件读取生成计划。
func LoadGenerationPlan(path string) (GenerationPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GenerationPlan{}, err
	}
	var plan GenerationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return GenerationPlan{}, fmt.Errorf("parse generation plan %s: %w", path, err)
	}
	return plan, ValidateGenerationPlan(plan)
}

// SaveGenerationPlan 校验并保存生成计划。
func SaveGenerationPlan(path string, plan GenerationPlan) error {
	if err := ValidateGenerationPlan(plan); err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// ValidateGenerationPlan 校验单表和多表生成计划的基本一致性。
func ValidateGenerationPlan(plan GenerationPlan) error {
	service := strings.TrimSpace(plan.ServiceName)
	if service == "" {
		return fmt.Errorf("generation plan service_name is required")
	}
	if _, err := splitServiceName(service); err != nil {
		return err
	}
	root := strings.TrimSpace(plan.RootTable)
	if root == "" {
		return fmt.Errorf("generation plan root_table is required")
	}
	if !tableNamePattern.MatchString(root) {
		return fmt.Errorf("generation plan root_table %q is invalid", root)
	}
	if len(plan.Tables) == 0 {
		return fmt.Errorf("generation plan tables is required")
	}
	tables := make(map[string]bool, len(plan.Tables))
	for _, table := range plan.Tables {
		table = strings.TrimSpace(table)
		if table == "" || !tableNamePattern.MatchString(table) {
			return fmt.Errorf("generation plan table %q is invalid", table)
		}
		tables[table] = true
	}
	if !tables[root] {
		return fmt.Errorf("generation plan root_table %q must be included in tables", root)
	}
	for _, relation := range plan.Relationships {
		if err := validateRelationship(relation, tables); err != nil {
			return err
		}
	}
	return nil
}

func validateRelationship(relation TableRelationship, tables map[string]bool) error {
	switch relation.Type {
	case RelationshipOneToOne, RelationshipOneToMany, RelationshipManyToOne, RelationshipManyToMany:
	default:
		return fmt.Errorf("generation plan relationship type %q is invalid", relation.Type)
	}
	if !tables[relation.FromTable] {
		return fmt.Errorf("generation plan relationship from_table %q is not selected", relation.FromTable)
	}
	if !tables[relation.ToTable] {
		return fmt.Errorf("generation plan relationship to_table %q is not selected", relation.ToTable)
	}
	for label, value := range map[string]string{
		"from_column": relation.FromColumn,
		"to_column":   relation.ToColumn,
	} {
		if strings.TrimSpace(value) == "" || !tableNamePattern.MatchString(value) {
			return fmt.Errorf("generation plan relationship %s %q is invalid", label, value)
		}
	}
	if relation.Type == RelationshipManyToMany {
		joinTable := strings.TrimSpace(relation.JoinTable)
		if joinTable == "" {
			return fmt.Errorf("generation plan many_to_many relationship requires join_table")
		}
		if !tableNamePattern.MatchString(joinTable) {
			return fmt.Errorf("generation plan relationship join_table %q is invalid", joinTable)
		}
	}
	return nil
}

func defaultPlanPath(root string, serviceName string) string {
	parts, err := splitServiceName(serviceName)
	if err != nil {
		return filepath.Join(root, "scaffold-plans", "service.json")
	}
	return filepath.Join(root, "scaffold-plans", strings.Join(parts, "_")+".json")
}
