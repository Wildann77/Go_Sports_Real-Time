package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"gorm.io/gorm/clause"
	"sports-dashboard/internal/core/config"
	coreDatabase "sports-dashboard/internal/core/database"
	"sports-dashboard/internal/core/exceptions"
	commentaryModels "sports-dashboard/internal/features/commentary/models"
	commentaryRepos "sports-dashboard/internal/features/commentary/repositories"
	commentarySchemas "sports-dashboard/internal/features/commentary/schemas"
	matchModels "sports-dashboard/internal/features/matches/models"
	matchRepos "sports-dashboard/internal/features/matches/repositories"
	"sports-dashboard/internal/shared/enums"
)

type fakeCommentaryState struct {
	match            *matchModels.Match
	commentaries     []*commentaryModels.Commentary
	nextCommentaryID int64
	activeTx         *fakeCommentaryTxState
	failCreate       error
	failSave         error
}

type fakeCommentaryTxState struct {
	match        *matchModels.Match
	commentaries []*commentaryModels.Commentary
}

type fakeCommentaryRepository struct {
	state *fakeCommentaryState
}

type fakeMatchRepository struct {
	state             *fakeCommentaryState
	lockedMatchIDs    []int64
	findByIDCalls     []int64
	saveWithTxInvoked int
}

type fakeBroadcaster struct {
	calls []broadcastCall
}

type broadcastCall struct {
	roomID    int64
	eventType string
	data      any
}

type fakeTransactionManager struct {
	state        *fakeCommentaryState
	beforeCommit func()
	committed    bool
	rolledBack   bool
}

func TestCreateCommentaryCommitPath(t *testing.T) {
	now := time.Now()
	state := &fakeCommentaryState{
		match: &matchModels.Match{
			ID:        10,
			HomeScore: 0,
			AwayScore: 0,
			Status:    string(enums.StatusScheduled),
			StartTime: now.Add(-time.Minute),
			EndTime:   now.Add(time.Hour),
		},
		nextCommentaryID: 100,
	}

	commentaryRepo := &fakeCommentaryRepository{state: state}
	matchRepo := &fakeMatchRepository{state: state}
	broadcaster := &fakeBroadcaster{}
	txManager := &fakeTransactionManager{
		state: state,
		beforeCommit: func() {
			if len(broadcaster.calls) != 0 {
				t.Fatalf("expected no broadcasts before commit, got %d", len(broadcaster.calls))
			}
		},
	}

	service := NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		broadcaster,
		txManager,
		&coreDatabase.TimeoutPolicy{},
	)

	req := &commentarySchemas.CreateCommentaryRequest{
		Minute:    12,
		EventType: "goal",
		Message:   " Home goal ",
		Payload: map[string]any{
			"homeScore": 1,
			"awayScore": 0,
		},
	}

	res, err := service.CreateCommentary(context.Background(), 10, req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !txManager.committed {
		t.Fatal("expected transaction to commit")
	}

	if len(state.commentaries) != 1 {
		t.Fatalf("expected 1 persisted commentary, got %d", len(state.commentaries))
	}

	if state.match.HomeScore != 1 || state.match.AwayScore != 0 {
		t.Fatalf("expected persisted score 1-0, got %d-%d", state.match.HomeScore, state.match.AwayScore)
	}

	if state.match.Status != string(enums.StatusLive) {
		t.Fatalf("expected persisted status %s, got %s", enums.StatusLive, state.match.Status)
	}

	if res.Message != "Home goal" {
		t.Fatalf("expected sanitized message, got %q", res.Message)
	}
	payload, ok := res.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected payload map, got %T", res.Payload)
	}
	if payload["homeScore"] != float64(1) || payload["awayScore"] != float64(0) {
		t.Fatalf("unexpected response payload %#v", payload)
	}

	if len(broadcaster.calls) != 2 {
		t.Fatalf("expected 2 broadcasts after commit, got %d", len(broadcaster.calls))
	}

	if broadcaster.calls[0].eventType != string(enums.WSEventCommentaryCreated) {
		t.Fatalf("expected first broadcast %s, got %s", enums.WSEventCommentaryCreated, broadcaster.calls[0].eventType)
	}

	if broadcaster.calls[1].eventType != string(enums.WSEventMatchUpdated) {
		t.Fatalf("expected second broadcast %s, got %s", enums.WSEventMatchUpdated, broadcaster.calls[1].eventType)
	}

	matchUpdateData, ok := broadcaster.calls[1].data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload for match update, got %T", broadcaster.calls[1].data)
	}

	if matchUpdateData["homeScore"] != float64(1) || matchUpdateData["awayScore"] != float64(0) {
		t.Fatalf("unexpected match update payload: %#v", matchUpdateData)
	}
}

func TestCreateCommentaryRejectsEmptyMessageAfterSanitization(t *testing.T) {
	service := NewCommentaryServiceWithDependencies(
		&fakeCommentaryRepository{state: &fakeCommentaryState{}},
		&fakeMatchRepository{state: &fakeCommentaryState{}},
		&fakeBroadcaster{},
		&fakeTransactionManager{state: &fakeCommentaryState{}},
		&coreDatabase.TimeoutPolicy{},
	)

	_, err := service.CreateCommentary(context.Background(), 10, &commentarySchemas.CreateCommentaryRequest{
		Minute:    12,
		EventType: "goal",
		Message:   "   \n\t ",
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
}

func TestCreateCommentaryPayloadWithoutScoreLeavesMatchUnchanged(t *testing.T) {
	now := time.Now()
	state := &fakeCommentaryState{
		match: &matchModels.Match{
			ID:        12,
			HomeScore: 2,
			AwayScore: 1,
			Status:    string(enums.StatusLive),
			StartTime: now.Add(-time.Minute),
			EndTime:   now.Add(time.Hour),
		},
		nextCommentaryID: 300,
	}

	commentaryRepo := &fakeCommentaryRepository{state: state}
	matchRepo := &fakeMatchRepository{state: state}
	broadcaster := &fakeBroadcaster{}
	txManager := &fakeTransactionManager{state: state}

	service := NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		broadcaster,
		txManager,
		&coreDatabase.TimeoutPolicy{},
	)

	res, err := service.CreateCommentary(context.Background(), 12, &commentarySchemas.CreateCommentaryRequest{
		Minute:    65,
		EventType: "foul",
		Message:   " Tactical foul ",
		Payload: map[string]any{
			"note": "yellow-card",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if state.match.HomeScore != 2 || state.match.AwayScore != 1 {
		t.Fatalf("expected score unchanged, got %d-%d", state.match.HomeScore, state.match.AwayScore)
	}
	if matchRepo.saveWithTxInvoked != 0 {
		t.Fatalf("expected no match save, got %d", matchRepo.saveWithTxInvoked)
	}
	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected only commentary.created broadcast, got %d", len(broadcaster.calls))
	}
	if broadcaster.calls[0].eventType != string(enums.WSEventCommentaryCreated) {
		t.Fatalf("expected %s, got %s", enums.WSEventCommentaryCreated, broadcaster.calls[0].eventType)
	}
	if res.Message != "Tactical foul" {
		t.Fatalf("expected sanitized message, got %q", res.Message)
	}
}

func TestCreateCommentaryRollbackPath(t *testing.T) {
	now := time.Now()
	state := &fakeCommentaryState{
		match: &matchModels.Match{
			ID:        11,
			HomeScore: 0,
			AwayScore: 0,
			Status:    string(enums.StatusLive),
			StartTime: now.Add(-time.Minute),
			EndTime:   now.Add(time.Hour),
		},
		nextCommentaryID: 200,
		failSave:         errors.New("save failed"),
	}

	commentaryRepo := &fakeCommentaryRepository{state: state}
	matchRepo := &fakeMatchRepository{state: state}
	broadcaster := &fakeBroadcaster{}
	txManager := &fakeTransactionManager{state: state}

	service := NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		broadcaster,
		txManager,
		&coreDatabase.TimeoutPolicy{},
	)

	req := &commentarySchemas.CreateCommentaryRequest{
		Minute:    55,
		EventType: "goal",
		Message:   "Away goal",
		Payload: map[string]any{
			"awayScore": 1,
		},
	}

	_, err := service.CreateCommentary(context.Background(), 11, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}

	if appErr.Code != exceptions.DATABASE_ERROR {
		t.Fatalf("expected %s, got %s", exceptions.DATABASE_ERROR, appErr.Code)
	}

	if !txManager.rolledBack {
		t.Fatal("expected transaction rollback")
	}

	if len(state.commentaries) != 0 {
		t.Fatalf("expected no persisted commentary after rollback, got %d", len(state.commentaries))
	}

	if state.match.HomeScore != 0 || state.match.AwayScore != 0 {
		t.Fatalf("expected score unchanged after rollback, got %d-%d", state.match.HomeScore, state.match.AwayScore)
	}

	if len(broadcaster.calls) != 0 {
		t.Fatalf("expected no broadcasts after rollback, got %d", len(broadcaster.calls))
	}
}

type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastToRoom(int64, string, any) {}

type failingMatchRepository struct {
	inner *matchRepos.MatchRepository
	err   error
}

func (r *failingMatchRepository) FindByID(ctx context.Context, id int64) (*matchModels.Match, error) {
	return r.inner.FindByID(ctx, id)
}

func (r *failingMatchRepository) FindByIDForUpdateWithTx(ctx context.Context, tx *gorm.DB, id int64) (*matchModels.Match, error) {
	return r.inner.FindByIDForUpdateWithTx(ctx, tx, id)
}

func (r *failingMatchRepository) SaveWithTx(context.Context, *gorm.DB, *matchModels.Match) error {
	return r.err
}

func TestCreateCommentaryWithRealDBPersistsCommitAndAtomicUpdates(t *testing.T) {
	db := openCommentaryServiceTestDB(t)
	resetCommentaryServiceTables(t, db)

	timeoutPolicy := newCommentaryServiceTimeoutPolicy()
	matchRepo := matchRepos.NewMatchRepository(db, timeoutPolicy)
	commentaryRepo := commentaryRepos.NewCommentaryRepository(db, timeoutPolicy)
	service := NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		noopBroadcaster{},
		NewGormTransactionManager(db),
		timeoutPolicy,
	)

	match := seedCommentaryServiceMatch(t, db, &matchModels.Match{
		Sport:     "football",
		HomeTeam:  "A",
		AwayTeam:  "B",
		HomeScore: 0,
		AwayScore: 0,
		Status:    string(enums.StatusScheduled),
		StartTime: time.Now().Add(-time.Minute),
		EndTime:   time.Now().Add(time.Hour),
	})

	_, err := service.CreateCommentary(context.Background(), match.ID, &commentarySchemas.CreateCommentaryRequest{
		Minute:    14,
		EventType: "goal",
		Message:   "Goal",
		Payload: map[string]any{
			"homeScore": 3,
			"awayScore": 2,
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var persistedMatch matchModels.Match
	if err := db.First(&persistedMatch, match.ID).Error; err != nil {
		t.Fatalf("failed to load match: %v", err)
	}
	if persistedMatch.HomeScore != 3 || persistedMatch.AwayScore != 2 {
		t.Fatalf("expected atomic score update 3-2, got %d-%d", persistedMatch.HomeScore, persistedMatch.AwayScore)
	}
	if persistedMatch.Status != string(enums.StatusLive) {
		t.Fatalf("expected status %s, got %s", enums.StatusLive, persistedMatch.Status)
	}

	var commentaries []commentaryModels.Commentary
	if err := db.Where("match_id = ?", match.ID).Find(&commentaries).Error; err != nil {
		t.Fatalf("failed to load commentary rows: %v", err)
	}
	if len(commentaries) != 1 {
		t.Fatalf("expected 1 commentary row, got %d", len(commentaries))
	}
}

func TestCreateCommentaryWithRealDBRollbackLeavesNoPartialData(t *testing.T) {
	db := openCommentaryServiceTestDB(t)
	resetCommentaryServiceTables(t, db)

	timeoutPolicy := newCommentaryServiceTimeoutPolicy()
	realMatchRepo := matchRepos.NewMatchRepository(db, timeoutPolicy)
	commentaryRepo := commentaryRepos.NewCommentaryRepository(db, timeoutPolicy)
	service := NewCommentaryServiceWithDependencies(
		commentaryRepo,
		&failingMatchRepository{inner: realMatchRepo, err: errors.New("forced save failure")},
		noopBroadcaster{},
		NewGormTransactionManager(db),
		timeoutPolicy,
	)

	match := seedCommentaryServiceMatch(t, db, &matchModels.Match{
		Sport:     "football",
		HomeTeam:  "A",
		AwayTeam:  "B",
		HomeScore: 0,
		AwayScore: 0,
		Status:    string(enums.StatusLive),
		StartTime: time.Now().Add(-time.Minute),
		EndTime:   time.Now().Add(time.Hour),
	})

	_, err := service.CreateCommentary(context.Background(), match.ID, &commentarySchemas.CreateCommentaryRequest{
		Minute:    25,
		EventType: "goal",
		Message:   "Goal",
		Payload: map[string]any{
			"homeScore": 1,
		},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var persistedMatch matchModels.Match
	if err := db.First(&persistedMatch, match.ID).Error; err != nil {
		t.Fatalf("failed to load match: %v", err)
	}
	if persistedMatch.HomeScore != 0 || persistedMatch.AwayScore != 0 {
		t.Fatalf("expected score unchanged after rollback, got %d-%d", persistedMatch.HomeScore, persistedMatch.AwayScore)
	}

	var count int64
	if err := db.Model(&commentaryModels.Commentary{}).Where("match_id = ?", match.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count commentary rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 commentary rows after rollback, got %d", count)
	}
}

func TestCreateCommentaryWithRealDBWaitsForRowLock(t *testing.T) {
	db := openCommentaryServiceTestDB(t)
	resetCommentaryServiceTables(t, db)

	timeoutPolicy := newCommentaryServiceTimeoutPolicy()
	matchRepo := matchRepos.NewMatchRepository(db, timeoutPolicy)
	commentaryRepo := commentaryRepos.NewCommentaryRepository(db, timeoutPolicy)
	service := NewCommentaryServiceWithDependencies(
		commentaryRepo,
		matchRepo,
		noopBroadcaster{},
		NewGormTransactionManager(db),
		timeoutPolicy,
	)

	match := seedCommentaryServiceMatch(t, db, &matchModels.Match{
		Sport:     "football",
		HomeTeam:  "A",
		AwayTeam:  "B",
		Status:    string(enums.StatusLive),
		StartTime: time.Now().Add(-time.Minute),
		EndTime:   time.Now().Add(time.Hour),
	})

	lockTx := db.Begin()
	if lockTx.Error != nil {
		t.Fatalf("failed to begin lock transaction: %v", lockTx.Error)
	}
	defer lockTx.Rollback()

	var locked matchModels.Match
	if err := lockTx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, match.ID).Error; err != nil {
		t.Fatalf("failed to lock match row: %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		close(started)
		_, err := service.CreateCommentary(context.Background(), match.ID, &commentarySchemas.CreateCommentaryRequest{
			Minute:    30,
			EventType: "shot",
			Message:   "Shot",
		})
		done <- err
	}()

	<-started
	time.Sleep(150 * time.Millisecond)

	select {
	case err := <-done:
		t.Fatalf("expected service to wait on row lock, finished early with %v", err)
	default:
	}

	if err := lockTx.Commit().Error; err != nil {
		t.Fatalf("failed to release row lock: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error after releasing row lock, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for locked commentary transaction to complete")
	}

	var count int64
	if err := db.Model(&commentaryModels.Commentary{}).Where("match_id = ?", match.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count commentary rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 commentary row after lock release, got %d", count)
	}
}

func openCommentaryServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	cfg := config.LoadConfig()
	db, err := coreDatabase.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("skipping commentary service integration test, db unavailable: %v", err)
	}

	if err := db.AutoMigrate(&matchModels.Match{}, &commentaryModels.Commentary{}); err != nil {
		t.Fatalf("failed to migrate commentary service tables: %v", err)
	}

	return db
}

func resetCommentaryServiceTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec("TRUNCATE TABLE commentary, matches RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatalf("failed to truncate commentary service tables: %v", err)
	}
}

func seedCommentaryServiceMatch(t *testing.T, db *gorm.DB, match *matchModels.Match) *matchModels.Match {
	t.Helper()

	if err := db.Create(match).Error; err != nil {
		t.Fatalf("failed to seed service match: %v", err)
	}

	return match
}

func newCommentaryServiceTimeoutPolicy() *coreDatabase.TimeoutPolicy {
	cfg := config.LoadConfig()
	return coreDatabase.NewTimeoutPolicy(cfg)
}

func (r *fakeCommentaryRepository) CreateWithTx(_ context.Context, _ *gorm.DB, c *commentaryModels.Commentary) error {
	if r.state.failCreate != nil {
		return r.state.failCreate
	}

	if r.state.activeTx == nil {
		return errors.New("commentary create without active tx")
	}

	r.state.nextCommentaryID++
	c.ID = r.state.nextCommentaryID
	c.CreatedAt = time.Now()
	r.state.activeTx.commentaries = append(r.state.activeTx.commentaries, cloneCommentary(c))
	return nil
}

func (r *fakeCommentaryRepository) FindByMatchID(_ context.Context, matchID int64, _ int) ([]*commentaryModels.Commentary, error) {
	if r.state.match == nil || r.state.match.ID != matchID {
		return []*commentaryModels.Commentary{}, nil
	}

	commentaries := make([]*commentaryModels.Commentary, 0, len(r.state.commentaries))
	for _, commentary := range r.state.commentaries {
		commentaries = append(commentaries, cloneCommentary(commentary))
	}

	return commentaries, nil
}

func (r *fakeMatchRepository) FindByID(_ context.Context, id int64) (*matchModels.Match, error) {
	r.findByIDCalls = append(r.findByIDCalls, id)
	if r.state.match == nil || r.state.match.ID != id {
		return nil, nil
	}
	return cloneMatch(r.state.match), nil
}

func (r *fakeMatchRepository) FindByIDForUpdateWithTx(_ context.Context, _ *gorm.DB, id int64) (*matchModels.Match, error) {
	r.lockedMatchIDs = append(r.lockedMatchIDs, id)
	if r.state.activeTx == nil {
		return nil, errors.New("match lock without active tx")
	}
	if r.state.activeTx.match == nil || r.state.activeTx.match.ID != id {
		return nil, nil
	}
	return r.state.activeTx.match, nil
}

func (r *fakeMatchRepository) SaveWithTx(_ context.Context, _ *gorm.DB, _ *matchModels.Match) error {
	r.saveWithTxInvoked++
	if r.state.failSave != nil {
		return r.state.failSave
	}
	if r.state.activeTx == nil {
		return errors.New("match save without active tx")
	}
	return nil
}

func (b *fakeBroadcaster) BroadcastToRoom(roomID int64, eventType string, data any) {
	b.calls = append(b.calls, broadcastCall{
		roomID:    roomID,
		eventType: eventType,
		data:      cloneBroadcastData(data),
	})
}

func (m *fakeTransactionManager) WithinTransaction(_ context.Context, fn func(tx *gorm.DB) error) error {
	m.state.activeTx = &fakeCommentaryTxState{
		match:        cloneMatch(m.state.match),
		commentaries: cloneCommentarySlice(m.state.commentaries),
	}

	err := fn(nil)
	if err != nil {
		m.rolledBack = true
		m.state.activeTx = nil
		return err
	}

	if m.beforeCommit != nil {
		m.beforeCommit()
	}

	m.state.match = m.state.activeTx.match
	m.state.commentaries = m.state.activeTx.commentaries
	m.state.activeTx = nil
	m.committed = true

	return nil
}

func cloneMatch(match *matchModels.Match) *matchModels.Match {
	if match == nil {
		return nil
	}

	cloned := *match
	return &cloned
}

func cloneCommentary(commentary *commentaryModels.Commentary) *commentaryModels.Commentary {
	if commentary == nil {
		return nil
	}

	cloned := *commentary
	if len(commentary.Payload) > 0 {
		cloned.Payload = append([]byte(nil), commentary.Payload...)
	}
	return &cloned
}

func cloneCommentarySlice(commentaries []*commentaryModels.Commentary) []*commentaryModels.Commentary {
	cloned := make([]*commentaryModels.Commentary, 0, len(commentaries))
	for _, commentary := range commentaries {
		cloned = append(cloned, cloneCommentary(commentary))
	}
	return cloned
}

func cloneBroadcastData(data any) any {
	payload, err := json.Marshal(data)
	if err != nil {
		return data
	}

	var cloned any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return data
	}

	return cloned
}
