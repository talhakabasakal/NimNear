package usecase

import (
	"context"
	"log/slog"
	"time"
)

// ReconciliationWorker periodically processes a bounded set of unresolved payment requests.
// It has no ownership of payment truth; the use case and verifier remain authoritative.
type ReconciliationWorker struct {
	useCase *RequestUseCase
	logger  *slog.Logger
}

func NewReconciliationWorker(useCase *RequestUseCase, logger *slog.Logger) *ReconciliationWorker {
	return &ReconciliationWorker{useCase: useCase, logger: logger}
}

// Run performs one pass immediately, then waits between bounded passes until shutdown.
func (w *ReconciliationWorker) Run(ctx context.Context) {
	if w == nil || w.useCase == nil {
		return
	}
	w.runOnce(ctx)
	ticker := time.NewTicker(w.useCase.reconciliation.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *ReconciliationWorker) runOnce(ctx context.Context) {
	report, err := w.useCase.Reconcile(ctx)
	if err != nil {
		if w.logger != nil {
			w.logger.Error("payment request reconciliation batch failed", "error", err)
		}
		return
	}
	if w.logger == nil {
		return
	}
	for _, result := range report.Results {
		level := slog.LevelInfo
		if result.Class == string(classificationTransient) || result.Class == string(classificationStateUpdateError) {
			level = slog.LevelWarn
		}
		retryable := result.After == modelStatusSubmitted || result.After == modelStatusVerifying
		w.logger.Log(ctx, level, "payment request reconciliation result",
			"public_id", result.PublicID,
			"request_id", result.RequestID,
			"state_before", result.Before,
			"state_after", result.After,
			"classification", result.Class,
			"retryable", retryable,
			"retry_interval_seconds", int64(w.useCase.reconciliation.Interval/time.Second),
		)
	}
	w.logger.Debug("payment request reconciliation batch completed", "selected", report.Selected, "processed", len(report.Results))
}

const (
	modelStatusSubmitted = "submitted"
	modelStatusVerifying = "verifying"
)
