package repositories

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	matchModels "sports-dashboard/internal/features/matches/models"
	"sports-dashboard/internal/features/matches/schemas"
)

func TestMatchRepositoryCreatePersistsRow(t *testing.T) {
	db := openMatchRepositoryTestDB(t)
	resetMatchesTable(t, db)

	repo := NewMatchRepository(db, newMatchTimeoutPolicy())
	match := &matchModels.Match{
		Sport:     "football",
		HomeTeam:  "Team A",
		AwayTeam:  "Team B",
		HomeScore: 1,
		AwayScore: 0,
		Status:    "live",
		StartTime: time.Now().Add(-time.Minute),
		EndTime:   time.Now().Add(time.Hour),
	}

	if err := repo.Create(context.Background(), match); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if match.ID == 0 {
		t.Fatal("expected created match ID to be assigned")
	}

	var persisted matchModels.Match
	if err := db.First(&persisted, match.ID).Error; err != nil {
		t.Fatalf("expected persisted row, got %v", err)
	}

	if persisted.Sport != "football" || persisted.HomeTeam != "Team A" || persisted.AwayTeam != "Team B" {
		t.Fatalf("unexpected persisted match: %#v", persisted)
	}
}

func TestMatchRepositoryFindByIDReturnsNilWhenNotFound(t *testing.T) {
	db := openMatchRepositoryTestDB(t)
	resetMatchesTable(t, db)

	repo := NewMatchRepository(db, newMatchTimeoutPolicy())

	match, err := repo.FindByID(context.Background(), 999999)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if match != nil {
		t.Fatalf("expected nil match, got %#v", match)
	}
}

func TestMatchRepositoryFindAllRespectsStatusFilterOrderingAndLimit(t *testing.T) {
	db := openMatchRepositoryTestDB(t)
	resetMatchesTable(t, db)

	repo := NewMatchRepository(db, newMatchTimeoutPolicy())
	now := time.Now()

	seeds := []*matchModels.Match{
		{
			Sport:     "football",
			HomeTeam:  "First",
			AwayTeam:  "One",
			Status:    "scheduled",
			StartTime: now.Add(time.Hour),
			EndTime:   now.Add(2 * time.Hour),
		},
		{
			Sport:     "football",
			HomeTeam:  "Second",
			AwayTeam:  "Two",
			Status:    "live",
			StartTime: now.Add(-time.Minute),
			EndTime:   now.Add(time.Hour),
		},
		{
			Sport:     "football",
			HomeTeam:  "Third",
			AwayTeam:  "Three",
			Status:    "scheduled",
			StartTime: now.Add(3 * time.Hour),
			EndTime:   now.Add(4 * time.Hour),
		},
	}

	for _, seed := range seeds {
		if err := repo.Create(context.Background(), seed); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	scheduledMatches, totalScheduled, err := repo.FindAll(context.Background(), schemas.ListMatchesQuery{Status: "scheduled", Limit: 10})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(scheduledMatches) != 2 {
		t.Fatalf("expected 2 scheduled matches, got %d", len(scheduledMatches))
	}
	if totalScheduled != 2 {
		t.Fatalf("expected totalScheduled to be 2, got %d", totalScheduled)
	}
	// Note: status sorting live first, scheduled next, finished last and sub-sort start_time DESC.
	// But default sorting is start_time DESC when Sort is empty/default.
	// So Third (startTime now+3h) comes before First (startTime now+1h).
	if scheduledMatches[0].HomeTeam != "Third" || scheduledMatches[1].HomeTeam != "First" {
		t.Fatalf("expected scheduled matches ordered by start_time desc, got %q then %q", scheduledMatches[0].HomeTeam, scheduledMatches[1].HomeTeam)
	}

	limitedMatches, totalAll, err := repo.FindAll(context.Background(), schemas.ListMatchesQuery{Limit: 2})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(limitedMatches) != 2 {
		t.Fatalf("expected 2 limited matches, got %d", len(limitedMatches))
	}
	if totalAll != 3 {
		t.Fatalf("expected totalAll to be 3, got %d", totalAll)
	}
	if limitedMatches[0].HomeTeam != "Third" || limitedMatches[1].HomeTeam != "First" {
		t.Fatalf("expected newest matches first by start_time, got %q then %q", limitedMatches[0].HomeTeam, limitedMatches[1].HomeTeam)
	}
}

func openMatchRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping repository integration test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&matchModels.Match{}); err != nil {
		t.Fatalf("failed to migrate matches table: %v", err)
	}

	return db
}

func resetMatchesTable(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE matches RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate matches table: %v", err)
	}
}

func newMatchTimeoutPolicy() *coreDatabase.TimeoutPolicy {
	cfg := config.LoadConfig()
	return coreDatabase.NewTimeoutPolicy(cfg)
}
