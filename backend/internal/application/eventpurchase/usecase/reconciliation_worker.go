package usecase

import (
	"context"
	"log/slog"
	"time"
)

// ReconciliationWorker periodically processes a bounded set of unresolved payments.
// It has no ownership of payment truth; the use case and verifier remain authoritative.
type ReconciliationWorker struct {
	useCase *PurchaseUseCase
	logger  *slog.Logger
}

func NewReconciliationWorker(useCase *PurchaseUseCase, logger *slog.Logger) *ReconciliationWorker {
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
			w.logger.Warn("payment reconciliation batch failed", "error", err)
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
		retryable := result.After == "submitted" || result.After == "verifying"
		w.logger.Log(ctx, level, "payment reconciliation result",
			"purchase_id", result.PurchaseID,
			"event_id", result.EventID,
			"state_before", result.Before,
			"state_after", result.After,
			"classification", result.Class,
			"retryable", retryable,
			"retry_interval_seconds", int64(w.useCase.reconciliation.Interval/time.Second),
		)
	}
	w.logger.Debug("payment reconciliation batch completed", "selected", report.Selected, "processed", len(report.Results))
}
