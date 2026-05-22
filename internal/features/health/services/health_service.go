package services

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"sports-dashboard/internal/core/exceptions"
)

const serviceName = "go-sports-realtime-api"

type SQLPinger interface {
	PingContext(ctx context.Context) error
}

type SQLDBProvider interface {
	DB() (SQLPinger, error)
}

type gormSQLDBProvider struct {
	db *gorm.DB
}

type Status struct {
	Status       string            `json:"status"`
	Service      string            `json:"service"`
	Check        string            `json:"check"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

type HealthService struct {
	dbProvider       SQLDBProvider
	readinessTimeout time.Duration
}

func NewGormDBProvider(db *gorm.DB) SQLDBProvider {
	return &gormSQLDBProvider{db: db}
}

func NewHealthService(dbProvider SQLDBProvider, readinessTimeout time.Duration) *HealthService {
	return &HealthService{
		dbProvider:       dbProvider,
		readinessTimeout: readinessTimeout,
	}
}

func (p *gormSQLDBProvider) DB() (SQLPinger, error) {
	if p.db == nil {
		return nil, fmt.Errorf("health readiness: gorm db is nil")
	}

	return p.db.DB()
}

func (s *HealthService) Liveness() Status {
	return Status{
		Status:  "ok",
		Service: serviceName,
		Check:   "liveness",
	}
}

func (s *HealthService) Readiness(ctx context.Context) (Status, error) {
	if s.dbProvider == nil {
		return s.readinessFailure(fmt.Errorf("health readiness: database provider is nil"))
	}

	sqlDB, err := s.dbProvider.DB()
	if err != nil {
		return s.readinessFailure(fmt.Errorf("health readiness: acquire sql db: %w", err))
	}

	pingCtx, cancel := context.WithTimeout(ctx, s.effectiveReadinessTimeout())
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		return s.readinessFailure(fmt.Errorf("health readiness: ping database: %w", err))
	}

	return Status{
		Status:  "ok",
		Service: serviceName,
		Check:   "readiness",
		Dependencies: map[string]string{
			"database": "ok",
		},
	}, nil
}

func (s *HealthService) effectiveReadinessTimeout() time.Duration {
	if s.readinessTimeout > 0 {
		return s.readinessTimeout
	}

	return 5 * time.Second
}

func (s *HealthService) readinessFailure(cause error) (Status, error) {
	return Status{
		Status:  "not_ready",
		Service: serviceName,
		Check:   "readiness",
		Dependencies: map[string]string{
			"database": "unavailable",
		},
	}, exceptions.NewServiceUnavailableError("Service is not ready", cause)
}
