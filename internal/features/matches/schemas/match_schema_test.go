package schemas

import (
	"github.com/go-playground/validator/v10"
	"sports-dashboard/internal/core/security"
	"testing"
	"time"
)

func TestCreateMatchValidation(t *testing.T) {
	validate := validator.New()
	validate.SetTagName("binding")
	security.RegisterCustomValidators(validate)

	tests := []struct {
		name    string
		req     CreateMatchRequest
		wantErr bool
	}{
		{
			name: "Valid Request",
			req: CreateMatchRequest{
				Sport:     "football",
				HomeTeam:  "Team A",
				AwayTeam:  "Team B",
				StartTime: time.Now(),
				EndTime:   time.Now().Add(1 * time.Hour),
				HomeScore: 0,
				AwayScore: 0,
			},
			wantErr: false,
		},
		{
			name: "Negative Score",
			req: CreateMatchRequest{
				Sport:     "football",
				HomeTeam:  "Team A",
				AwayTeam:  "Team B",
				StartTime: time.Now(),
				EndTime:   time.Now().Add(1 * time.Hour),
				HomeScore: -1,
				AwayScore: 0,
			},
			wantErr: true,
		},
		{
			name: "Missing Required Fields",
			req: CreateMatchRequest{
				HomeTeam: "Team A",
			},
			wantErr: true,
		},
		{
			name: "Invalid Empty White Space String",
			req: CreateMatchRequest{
				Sport:     "football",
				HomeTeam:  "    ",
				AwayTeam:  "Team B",
				StartTime: time.Now(),
				EndTime:   time.Now().Add(1 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "Invalid Safe Slug",
			req: CreateMatchRequest{
				Sport:     "Football Match 123", // invalid
				HomeTeam:  "Team A",
				AwayTeam:  "Team B",
				StartTime: time.Now(),
				EndTime:   time.Now().Add(1 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate struct error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
