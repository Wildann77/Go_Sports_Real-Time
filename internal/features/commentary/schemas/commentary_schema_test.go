package schemas

import (
	"testing"

	"github.com/go-playground/validator/v10"

	"sports-dashboard/internal/core/security"
)

func TestCreateCommentaryValidation(t *testing.T) {
	validate := validator.New()
	validate.SetTagName("binding")
	security.RegisterCustomValidators(validate)

	tests := []struct {
		name    string
		req     CreateCommentaryRequest
		wantErr bool
	}{
		{
			name: "valid request with score payload",
			req: CreateCommentaryRequest{
				Minute:    12,
				EventType: "goal",
				Message:   "Home goal",
				Payload: map[string]any{
					"homeScore": 1,
					"awayScore": 0,
				},
			},
			wantErr: false,
		},
		{
			name: "rejects negative minute",
			req: CreateCommentaryRequest{
				Minute:    -1,
				EventType: "goal",
				Message:   "Goal",
			},
			wantErr: true,
		},
		{
			name: "rejects whitespace-only message",
			req: CreateCommentaryRequest{
				Minute:    12,
				EventType: "goal",
				Message:   "   \n\t ",
			},
			wantErr: true,
		},
		{
			name: "rejects non-object payload",
			req: CreateCommentaryRequest{
				Minute:    12,
				EventType: "goal",
				Message:   "Goal",
				Payload:   []string{"bad"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate struct error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
