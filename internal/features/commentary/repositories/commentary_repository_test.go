package repositories

import (
	"context"
	"testing"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	matchModels "sports-dashboard/internal/features/matches/models"
)

func TestCommentaryRepositoryCreateWithTxPersistsRow(t *testing.T) {
	db := openCommentaryRepositoryTestDB(t)
	resetCommentaryRepositoryTables(t, db)

	repo := NewCommentaryRepository(db, newCommentaryTimeoutPolicy())
	matchID := seedCommentaryMatch(t, db)

	err := db.Transaction(func(tx *gorm.DB) error {
		return repo.CreateWithTx(context.Background(), tx, &commentaryModels.Commentary{
			MatchID:   matchID,
			Minute:    5,
			EventType: "kickoff",
			Message:   "Kickoff",
			Payload:   datatypes.JSON([]byte(`{"phase":"start"}`)),
		})
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var persisted commentaryModels.Commentary
	if err := db.First(&persisted).Error; err != nil {
		t.Fatalf("expected persisted commentary row, got %v", err)
	}

	if persisted.MatchID != matchID || persisted.EventType != "kickoff" {
		t.Fatalf("unexpected persisted commentary %#v", persisted)
	}
}

func TestCommentaryRepositoryFindByMatchIDRespectsOrderingAndLimit(t *testing.T) {
	db := openCommentaryRepositoryTestDB(t)
	resetCommentaryRepositoryTables(t, db)

	repo := NewCommentaryRepository(db, newCommentaryTimeoutPolicy())
	matchID := seedCommentaryMatch(t, db)

	seeds := []*commentaryModels.Commentary{
		{
			MatchID:   matchID,
			Minute:    1,
			EventType: "kickoff",
			Message:   "Start",
		},
		{
			MatchID:   matchID,
			Minute:    10,
			EventType: "chance",
			Message:   "Chance",
		},
		{
			MatchID:   matchID,
			Minute:    20,
			EventType: "goal",
			Message:   "Goal",
		},
	}

	for _, seed := range seeds {
		if err := db.Create(seed).Error; err != nil {
			t.Fatalf("failed to seed commentary: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	commentaries, err := repo.FindByMatchID(context.Background(), matchID, 2)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(commentaries) != 2 {
		t.Fatalf("expected 2 commentaries, got %d", len(commentaries))
	}
	if commentaries[0].Message != "Start" || commentaries[1].Message != "Chance" {
		t.Fatalf("expected created_at asc ordering, got %q then %q", commentaries[0].Message, commentaries[1].Message)
	}
}

func openCommentaryRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping commentary repository integration test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&matchModels.Match{}, &commentaryModels.Commentary{}); err != nil {
		t.Fatalf("failed to migrate commentary repository tables: %v", err)
	}

	return db
}

func resetCommentaryRepositoryTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE commentary, matches RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate commentary repository tables: %v", err)
	}
}

func seedCommentaryMatch(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	match := &matchModels.Match{
		Sport:     "football",
		HomeTeam:  "Team A",
		AwayTeam:  "Team B",
		Status:    "live",
		StartTime: time.Now().Add(-time.Minute),
		EndTime:   time.Now().Add(time.Hour),
	}
	if err := db.Create(match).Error; err != nil {
		t.Fatalf("failed to seed match: %v", err)
	}

	return match.ID
}

func newCommentaryTimeoutPolicy() *coreDatabase.TimeoutPolicy {
	cfg := config.LoadConfig()
	return coreDatabase.NewTimeoutPolicy(cfg)
}
