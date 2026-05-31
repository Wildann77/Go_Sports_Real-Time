package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/datatypes"

	"sports-dashboard/internal/core/exceptions"
	matchModels "sports-dashboard/internal/features/matches/models"
	matchSchemas "sports-dashboard/internal/features/matches/schemas"
	"sports-dashboard/internal/shared/enums"
)

type fakeMatchRepository struct {
	createFn   func(ctx context.Context, match *matchModels.Match) error
	findAllFn  func(ctx context.Context, query matchSchemas.ListMatchesQuery) ([]*matchModels.Match, int64, error)
	findByIDFn func(ctx context.Context, id int64) (*matchModels.Match, error)
}

func (f *fakeMatchRepository) Create(ctx context.Context, match *matchModels.Match) error {
	if f.createFn != nil {
		return f.createFn(ctx, match)
	}
	return nil
}

func (f *fakeMatchRepository) FindAll(ctx context.Context, query matchSchemas.ListMatchesQuery) ([]*matchModels.Match, int64, error) {
	if f.findAllFn != nil {
		return f.findAllFn(ctx, query)
	}
	return []*matchModels.Match{}, 0, nil
}

func (f *fakeMatchRepository) FindByID(ctx context.Context, id int64) (*matchModels.Match, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, id)
	}
	return nil, nil
}

func TestMatchServiceCreateMatchValidPathAndStatusDerivation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		startTime      time.Time
		endTime        time.Time
		expectedStatus string
	}{
		{
			name:           "scheduled match",
			startTime:      now.Add(time.Hour),
			endTime:        now.Add(2 * time.Hour),
			expectedStatus: string(enums.StatusScheduled),
		},
		{
			name:           "live match",
			startTime:      now.Add(-time.Minute),
			endTime:        now.Add(time.Hour),
			expectedStatus: string(enums.StatusLive),
		},
		{
			name:           "finished match",
			startTime:      now.Add(-2 * time.Hour),
			endTime:        now.Add(-time.Hour),
			expectedStatus: string(enums.StatusFinished),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var created *matchModels.Match

			service := NewMatchService(&fakeMatchRepository{
				createFn: func(_ context.Context, match *matchModels.Match) error {
					created = match
					match.ID = 77
					match.CreatedAt = now
					match.UpdatedAt = now
					return nil
				},
			})

			res, err := service.CreateMatch(context.Background(), &matchSchemas.CreateMatchRequest{
				Sport:     "Football League",
				HomeTeam:  "  Team A  ",
				AwayTeam:  " Team B ",
				StartTime: tt.startTime,
				EndTime:   tt.endTime,
				Metadata:  nil,
			})
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}

			if created == nil {
				t.Fatal("expected repository create to be called")
			}
			if created.Sport != "footballleague" {
				t.Fatalf("expected sanitized sport footballleague, got %q", created.Sport)
			}
			if created.HomeTeam != "Team A" || created.AwayTeam != "Team B" {
				t.Fatalf("expected sanitized teams, got %q vs %q", created.HomeTeam, created.AwayTeam)
			}
			if created.Status != tt.expectedStatus {
				t.Fatalf("expected status %q, got %q", tt.expectedStatus, created.Status)
			}
			if string(created.Metadata) != "{}" {
				t.Fatalf("expected default metadata {}, got %s", string(created.Metadata))
			}
			if res.Status != tt.expectedStatus {
				t.Fatalf("expected response status %q, got %q", tt.expectedStatus, res.Status)
			}
		})
	}
}

func TestMatchServiceCreateMatchRejectsInvalidSport(t *testing.T) {
	createCalled := false
	service := NewMatchService(&fakeMatchRepository{
		createFn: func(_ context.Context, _ *matchModels.Match) error {
			createCalled = true
			return nil
		},
	})

	_, err := service.CreateMatch(context.Background(), &matchSchemas.CreateMatchRequest{
		Sport:     "!!!",
		HomeTeam:  "Team A",
		AwayTeam:  "Team B",
		StartTime: time.Now().Add(time.Hour),
		EndTime:   time.Now().Add(2 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != exceptions.VALIDATION_ERROR {
		t.Fatalf("expected %s, got %s", exceptions.VALIDATION_ERROR, appErr.Code)
	}
	if createCalled {
		t.Fatal("expected repository create not to be called")
	}
}

func TestMatchServiceCreateMatchRejectsStartTimeAfterEndTime(t *testing.T) {
	createCalled := false
	service := NewMatchService(&fakeMatchRepository{
		createFn: func(_ context.Context, _ *matchModels.Match) error {
			createCalled = true
			return nil
		},
	})

	_, err := service.CreateMatch(context.Background(), &matchSchemas.CreateMatchRequest{
		Sport:     "football",
		HomeTeam:  "Team A",
		AwayTeam:  "Team B",
		StartTime: time.Now().Add(2 * time.Hour),
		EndTime:   time.Now().Add(time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != exceptions.VALIDATION_ERROR {
		t.Fatalf("expected %s, got %s", exceptions.VALIDATION_ERROR, appErr.Code)
	}
	if createCalled {
		t.Fatal("expected repository create not to be called")
	}
}

func TestMatchServiceGetMatchesRejectsInvalidStatusFilter(t *testing.T) {
	service := NewMatchService(&fakeMatchRepository{})

	_, _, err := service.GetMatches(context.Background(), matchSchemas.ListMatchesQuery{Status: "paused", Limit: 10})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != exceptions.VALIDATION_ERROR {
		t.Fatalf("expected %s, got %s", exceptions.VALIDATION_ERROR, appErr.Code)
	}
}

func TestMatchServiceGetMatchReturnsNotFound(t *testing.T) {
	service := NewMatchService(&fakeMatchRepository{
		findByIDFn: func(_ context.Context, id int64) (*matchModels.Match, error) {
			return nil, nil
		},
	})

	_, err := service.GetMatch(context.Background(), 999)
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != exceptions.NOT_FOUND {
		t.Fatalf("expected %s, got %s", exceptions.NOT_FOUND, appErr.Code)
	}
}

func TestMatchServiceGetMatchMapsMetadata(t *testing.T) {
	now := time.Now()
	service := NewMatchService(&fakeMatchRepository{
		findByIDFn: func(_ context.Context, id int64) (*matchModels.Match, error) {
			return &matchModels.Match{
				ID:        id,
				Sport:     "football",
				HomeTeam:  "A",
				AwayTeam:  "B",
				Status:    string(enums.StatusLive),
				StartTime: now.Add(-time.Minute),
				EndTime:   now.Add(time.Hour),
				Metadata:  datatypes.JSON([]byte(`{"venue":"stadium"}`)),
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	})

	res, err := service.GetMatch(context.Background(), 7)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	metadata, ok := res.Metadata.(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata map, got %T", res.Metadata)
	}
	if metadata["venue"] != "stadium" {
		t.Fatalf("expected venue stadium, got %#v", metadata)
	}
}
