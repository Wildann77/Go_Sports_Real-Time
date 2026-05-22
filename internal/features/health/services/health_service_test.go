package services

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	"sports-dashboard/internal/core/exceptions"
)

type fakeSQLDBProvider struct {
	pinger SQLPinger
	err    error
}

type fakeSQLPinger struct {
	err error
}

type observingSQLPinger struct {
	deadlineSet bool
	deadline    time.Time
}

func (f *fakeSQLDBProvider) DB() (SQLPinger, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.pinger, nil
}

func (f *fakeSQLPinger) PingContext(context.Context) error {
	return f.err
}

func (o *observingSQLPinger) PingContext(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	o.deadlineSet = ok
	o.deadline = deadline
	<-ctx.Done()
	return ctx.Err()
}

func TestHealthServiceLiveness(t *testing.T) {
	svc := NewHealthService(nil, time.Second)

	status := svc.Liveness()

	if status.Status != "ok" {
		t.Fatalf("expected ok status, got %s", status.Status)
	}
	if status.Check != "liveness" {
		t.Fatalf("expected liveness check, got %s", status.Check)
	}
	if status.Service == "" {
		t.Fatal("expected service name to be set")
	}
}

func TestHealthServiceReadinessSuccess(t *testing.T) {
	svc := NewHealthService(&fakeSQLDBProvider{
		pinger: &fakeSQLPinger{},
	}, time.Second)

	status, err := svc.Readiness(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if status.Status != "ok" {
		t.Fatalf("expected ok status, got %s", status.Status)
	}

	if status.Dependencies["database"] != "ok" {
		t.Fatalf("expected database dependency to be ok, got %s", status.Dependencies["database"])
	}
}

func TestHealthServiceReadinessProviderError(t *testing.T) {
	svc := NewHealthService(&fakeSQLDBProvider{
		err: errors.New("db unavailable"),
	}, time.Second)

	_, err := svc.Readiness(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}

	if appErr.Code != exceptions.SERVICE_UNAVAILABLE {
		t.Fatalf("expected %s, got %s", exceptions.SERVICE_UNAVAILABLE, appErr.Code)
	}
}

func TestHealthServiceReadinessNilProvider(t *testing.T) {
	svc := NewHealthService(nil, time.Second)

	_, err := svc.Readiness(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}

	if appErr.Code != exceptions.SERVICE_UNAVAILABLE {
		t.Fatalf("expected %s, got %s", exceptions.SERVICE_UNAVAILABLE, appErr.Code)
	}
}

func TestHealthServiceReadinessPingError(t *testing.T) {
	svc := NewHealthService(&fakeSQLDBProvider{
		pinger: &fakeSQLPinger{err: context.DeadlineExceeded},
	}, time.Second)

	_, err := svc.Readiness(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}

	if appErr.Code != exceptions.SERVICE_UNAVAILABLE {
		t.Fatalf("expected %s, got %s", exceptions.SERVICE_UNAVAILABLE, appErr.Code)
	}
}

func TestHealthServiceReadinessUsesBoundedTimeout(t *testing.T) {
	pinger := &observingSQLPinger{}
	timeout := 25 * time.Millisecond
	svc := NewHealthService(&fakeSQLDBProvider{pinger: pinger}, timeout)

	start := time.Now()
	_, err := svc.Readiness(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !pinger.deadlineSet {
		t.Fatal("expected readiness ping context to have deadline")
	}

	deadlineDelta := pinger.deadline.Sub(start)
	if deadlineDelta <= 0 || deadlineDelta > 200*time.Millisecond {
		t.Fatalf("expected bounded deadline near timeout, got %s", deadlineDelta)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected readiness to fail quickly, got %s", elapsed)
	}

	var appErr *exceptions.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != exceptions.SERVICE_UNAVAILABLE {
		t.Fatalf("expected %s, got %s", exceptions.SERVICE_UNAVAILABLE, appErr.Code)
	}
}

func TestHealthServiceReadinessFailureStatusContract(t *testing.T) {
	svc := NewHealthService(&fakeSQLDBProvider{
		pinger: &fakeSQLPinger{err: context.DeadlineExceeded},
	}, time.Second)

	status, err := svc.Readiness(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if status.Status != "not_ready" {
		t.Fatalf("expected not_ready status, got %s", status.Status)
	}
	if status.Check != "readiness" {
		t.Fatalf("expected readiness check, got %s", status.Check)
	}
	if status.Dependencies["database"] != "unavailable" {
		t.Fatalf("expected database unavailable, got %s", status.Dependencies["database"])
	}

	payload, marshalErr := json.Marshal(status)
	if marshalErr != nil {
		t.Fatalf("expected status to be serializable, got %v", marshalErr)
	}
	if len(payload) == 0 {
		t.Fatal("expected non-empty serialized status")
	}
}

func TestHealthServiceReadinessFailureDoesNotLeakGoroutines(t *testing.T) {
	pinger := &observingSQLPinger{}
	svc := NewHealthService(&fakeSQLDBProvider{pinger: pinger}, 10*time.Millisecond)

	baseline := runtime.NumGoroutine()

	for i := 0; i < 10; i++ {
		_, err := svc.Readiness(context.Background())
		if err == nil {
			t.Fatal("expected readiness failure, got nil")
		}
	}

	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > baseline+2 {
		t.Fatalf("expected no goroutine leak after repeated readiness failures, baseline=%d after=%d", baseline, after)
	}
}
