package security

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestRegisterCustomValidatorsNonEmptyTrimmed(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	RegisterCustomValidators(validate)

	type input struct {
		Value string `validate:"non_empty_trimmed"`
	}

	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name:    "accepts trimmed non-empty value",
			input:   input{Value: "  live  "},
			wantErr: false,
		},
		{
			name:    "rejects whitespace-only value",
			input:   input{Value: "   \n\t  "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}
}

func TestRegisterCustomValidatorsSafeSlug(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	RegisterCustomValidators(validate)

	type input struct {
		Slug string `validate:"safe_slug"`
	}

	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name:    "accepts lowercase slug with separators",
			input:   input{Slug: "liga-1_group-a"},
			wantErr: false,
		},
		{
			name:    "rejects uppercase and spaces",
			input:   input{Slug: "Liga 1"},
			wantErr: true,
		},
		{
			name:    "rejects special characters",
			input:   input{Slug: "liga/1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}
}

func TestRegisterCustomValidatorsJSONObject(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	RegisterCustomValidators(validate)

	type input struct {
		Payload any `validate:"json_object"`
	}

	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name:    "accepts object payload",
			input:   input{Payload: map[string]any{"homeScore": 2, "note": "goal"}},
			wantErr: false,
		},
		{
			name:    "accepts nil map payload",
			input:   input{Payload: map[string]any(nil)},
			wantErr: false,
		},
		{
			name:    "rejects array payload",
			input:   input{Payload: []string{"a", "b"}},
			wantErr: true,
		},
		{
			name:    "rejects primitive payload",
			input:   input{Payload: "not-an-object"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}
}

func TestRegisterCustomValidatorsRejectInvalidEnums(t *testing.T) {
	t.Parallel()

	validate := validator.New()
	RegisterCustomValidators(validate)

	type input struct {
		Status    string `validate:"match_status"`
		EventType string `validate:"ws_event_type"`
	}

	tests := []struct {
		name    string
		input   input
		wantErr bool
	}{
		{
			name:    "accepts valid enum values",
			input:   input{Status: "live", EventType: "match.updated"},
			wantErr: false,
		},
		{
			name:    "rejects invalid match status",
			input:   input{Status: "paused", EventType: "match.updated"},
			wantErr: true,
		},
		{
			name:    "rejects invalid websocket event type",
			input:   input{Status: "scheduled", EventType: "match.deleted"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no validation error, got %v", err)
			}
		})
	}
}
