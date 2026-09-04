package entity

import (
	"fmt"
	"regexp"
	"strings"
)

// FieldType represents supported entity field types
type FieldType string

const (
	TypeUUID     FieldType = "UUID"
	TypeString   FieldType = "String"
	TypeEmail    FieldType = "Email"
	TypeInt      FieldType = "Int"
	TypeFloat    FieldType = "Float"
	TypeBool     FieldType = "Bool"
	TypeMarkdown FieldType = "Markdown"
	TypeDate     FieldType = "Date"
	TypeRelation FieldType = "Relation"
)

// Field represents an attribute in an Entity model
type Field struct {
	Name         string    `json:"name"`
	Type         FieldType `json:"type"`
	IsPrimaryKey bool      `json:"is_primary_key"`
	IsRequired   bool      `json:"is_required"`
	IsUnique     bool      `json:"is_unique"`
	TargetEntity string    `json:"target_entity,omitempty"` // For relations
}

// Entity represents a Unified Application Model
type Entity struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

// NewEntity creates a new entity definition
func NewEntity(name string) *Entity {
	return &Entity{
		Name:   name,
		Fields: []Field{},
	}
}

// AddField adds a field to the entity
func (e *Entity) AddField(name string, fieldType FieldType, required bool) *Entity {
	f := Field{
		Name:         name,
		Type:         fieldType,
		IsRequired:   required,
		IsPrimaryKey: strings.EqualFold(name, "id"),
	}
	e.Fields = append(e.Fields, f)
	return e
}

// AddRelation adds a relationship to another entity
func (e *Entity) AddRelation(name string, targetEntity string, required bool) *Entity {
	f := Field{
		Name:         name,
		Type:         TypeRelation,
		IsRequired:   required,
		TargetEntity: targetEntity,
	}
	e.Fields = append(e.Fields, f)
	return e
}

// ─── SQL DDL GENERATOR ──────────────────────────────────────────────────────

// GenerateSQL produces standard CREATE TABLE DDL
func (e *Entity) GenerateSQL(dialect string) string {
	var sb strings.Builder
	tableName := strings.ToLower(e.Name) + "s"

	sb.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", tableName))

	var colDefs []string
	for _, f := range e.Fields {
		sqlType := "TEXT"
		switch f.Type {
		case TypeUUID:
			if strings.ToLower(dialect) == "postgres" {
				sqlType = "UUID"
			} else {
				sqlType = "TEXT"
			}
		case TypeString, TypeEmail, TypeMarkdown:
			sqlType = "VARCHAR(255)"
			if f.Type == TypeMarkdown {
				sqlType = "TEXT"
			}
		case TypeInt:
			sqlType = "INTEGER"
		case TypeFloat:
			sqlType = "DOUBLE PRECISION"
		case TypeBool:
			sqlType = "BOOLEAN"
		case TypeDate:
			sqlType = "TIMESTAMP WITH TIME ZONE"
		case TypeRelation:
			sqlType = "UUID" // Foreign key ID
		}

		def := fmt.Sprintf("    %s %s", strings.ToLower(f.Name), sqlType)
		if f.IsPrimaryKey {
			def += " PRIMARY KEY"
		} else if f.IsRequired {
			def += " NOT NULL"
		}
		if f.IsUnique {
			def += " UNIQUE"
		}
		if f.Type == TypeRelation && f.TargetEntity != "" {
			targetTable := strings.ToLower(f.TargetEntity) + "s"
			def += fmt.Sprintf(" REFERENCES %s(id)", targetTable)
		}
		colDefs = append(colDefs, def)
	}

	sb.WriteString(strings.Join(colDefs, ",\n"))
	sb.WriteString("\n);")
	return sb.String()
}

// ─── REST API SPEC GENERATOR ────────────────────────────────────────────────

type EndpointSpec struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// GenerateRESTEndpoints produces REST API endpoint specifications
func (e *Entity) GenerateRESTEndpoints() []EndpointSpec {
	basePath := "/" + strings.ToLower(e.Name) + "s"
	return []EndpointSpec{
		{Method: "GET", Path: basePath, Description: fmt.Sprintf("List all %ss", e.Name)},
		{Method: "POST", Path: basePath, Description: fmt.Sprintf("Create a new %s", e.Name)},
		{Method: "GET", Path: basePath + "/{id}", Description: fmt.Sprintf("Get %s by ID", e.Name)},
		{Method: "PUT", Path: basePath + "/{id}", Description: fmt.Sprintf("Update %s", e.Name)},
		{Method: "DELETE", Path: basePath + "/{id}", Description: fmt.Sprintf("Delete %s", e.Name)},
	}
}

// ─── CLIENT MODEL GENERATOR ─────────────────────────────────────────────────

// GenerateClientModel generates typed model code for web/mobile
func (e *Entity) GenerateClientModel(target string) string {
	var sb strings.Builder
	switch strings.ToLower(target) {
	case "web", "typescript":
		sb.WriteString(fmt.Sprintf("export interface %s {\n", e.Name))
		for _, f := range e.Fields {
			tsType := "string"
			switch f.Type {
			case TypeInt, TypeFloat:
				tsType = "number"
			case TypeBool:
				tsType = "boolean"
			case TypeRelation:
				tsType = f.TargetEntity
			}
			opt := ""
			if !f.IsRequired {
				opt = "?"
			}
			sb.WriteString(fmt.Sprintf("  %s%s: %s;\n", f.Name, opt, tsType))
		}
		sb.WriteString("}\n")

	case "mobile", "nillang":
		sb.WriteString(fmt.Sprintf("struct %s {\n", e.Name))
		for _, f := range e.Fields {
			nilType := string(f.Type)
			if f.Type == TypeRelation {
				nilType = f.TargetEntity
			}
			sb.WriteString(fmt.Sprintf("    %s: %s\n", f.Name, nilType))
		}
		sb.WriteString("}\n")
	}

	return sb.String()
}

// ─── VALIDATION ENGINE ──────────────────────────────────────────────────────

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Validate checks an entity instance map against the schema
func (e *Entity) Validate(data map[string]interface{}) []string {
	var errors []string

	for _, f := range e.Fields {
		val, exists := data[f.Name]
		if !exists || val == nil {
			if f.IsRequired {
				errors = append(errors, fmt.Sprintf("field %q is required", f.Name))
			}
			continue
		}

		switch f.Type {
		case TypeString, TypeMarkdown, TypeUUID:
			if _, ok := val.(string); !ok {
				errors = append(errors, fmt.Sprintf("field %q must be a string", f.Name))
			}
		case TypeEmail:
			s, ok := val.(string)
			if !ok || !emailRegex.MatchString(s) {
				errors = append(errors, fmt.Sprintf("field %q must be a valid email address", f.Name))
			}
		case TypeInt:
			switch val.(type) {
			case int, int64, int32:
			default:
				errors = append(errors, fmt.Sprintf("field %q must be an integer", f.Name))
			}
		case TypeBool:
			if _, ok := val.(bool); !ok {
				errors = append(errors, fmt.Sprintf("field %q must be a boolean", f.Name))
			}
		}
	}

	return errors
}
