package entity

import (
	"strings"
	"testing"
)

func TestEntitySchemaGenerators(t *testing.T) {
	// User entity matching Section 8 of refactor.md
	user := NewEntity("User").
		AddField("id", TypeUUID, true).
		AddField("name", TypeString, true).
		AddField("email", TypeEmail, true)

	// Post entity with relationship to User
	post := NewEntity("Post").
		AddField("id", TypeUUID, true).
		AddRelation("author", "User", true).
		AddField("title", TypeString, true).
		AddField("body", TypeMarkdown, false)

	// 1. Test SQL Generation
	userSQL := user.GenerateSQL("postgres")
	if !strings.Contains(userSQL, "CREATE TABLE IF NOT EXISTS users") ||
		!strings.Contains(userSQL, "id UUID PRIMARY KEY") ||
		!strings.Contains(userSQL, "email VARCHAR(255) NOT NULL") {
		t.Errorf("User SQL generation failed:\n%s", userSQL)
	}

	postSQL := post.GenerateSQL("postgres")
	if !strings.Contains(postSQL, "REFERENCES users(id)") {
		t.Errorf("Post SQL foreign key generation failed:\n%s", postSQL)
	}

	// 2. Test REST API Endpoints
	endpoints := user.GenerateRESTEndpoints()
	if len(endpoints) != 5 {
		t.Errorf("expected 5 REST endpoints, got %d", len(endpoints))
	}
	if endpoints[0].Path != "/users" || endpoints[0].Method != "GET" {
		t.Errorf("first endpoint invalid: %v", endpoints[0])
	}

	// 3. Test Client Model Generation
	tsModel := user.GenerateClientModel("web")
	if !strings.Contains(tsModel, "export interface User") ||
		!strings.Contains(tsModel, "email: string;") {
		t.Errorf("TypeScript model generation failed:\n%s", tsModel)
	}

	nilModel := user.GenerateClientModel("nillang")
	if !strings.Contains(nilModel, "struct User") ||
		!strings.Contains(nilModel, "email: Email") {
		t.Errorf("NilLang model generation failed:\n%s", nilModel)
	}

	// 4. Test Validation Engine
	validData := map[string]interface{}{
		"id":    "550e8400-e29b-41d4-a716-446655440000",
		"name":  "Joysriram",
		"email": "joysriram@example.com",
	}
	errs := user.Validate(validData)
	if len(errs) != 0 {
		t.Errorf("expected valid data, got errors: %v", errs)
	}

	invalidData := map[string]interface{}{
		"id":    "550e8400-e29b-41d4-a716-446655440000",
		"name":  "Joysriram",
		"email": "not-an-email",
	}
	errsInvalid := user.Validate(invalidData)
	if len(errsInvalid) == 0 {
		t.Errorf("expected validation errors for invalid email, got none")
	}
}
