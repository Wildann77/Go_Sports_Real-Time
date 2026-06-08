package database

import (
	"context"
	"time"

	"sports-dashboard/internal/core/config"
)

type TimeoutPolicy struct {
	queryTimeout time.Duration
	txTimeout    time.Duration
}

func NewTimeoutPolicy(cfg *config.Config) *TimeoutPolicy {
	return &TimeoutPolicy{
		queryTimeout: cfg.DBQueryTimeout(),
		txTimeout:    cfg.DBTxTimeout(),
	}
}

func (p *TimeoutPolicy) WithQueryTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if p == nil || p.queryTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, p.queryTimeout)
}

func (p *TimeoutPolicy) WithTransactionTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if p == nil || p.txTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, p.txTimeout)
}
