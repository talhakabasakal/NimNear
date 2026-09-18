package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	purchaseUC "github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/usecase"
	paymentRequestUC "github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/usecase"
	nimiqRPC "github.com/masterfabric-go/masterfabric/internal/infrastructure/nimiq/rpc"
	pgEventPurchase "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/eventpurchase"
	pgIam "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/iam"
	pgPaymentRequest "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/paymentrequest"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
	"github.com/masterfabric-go/masterfabric/internal/shared/logger"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "recover-payment: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: recover-payment <event-purchase|payment-request> <id>")
	}
	domain := strings.ToLower(strings.TrimSpace(args[0]))
	id, err := uuid.Parse(strings.TrimSpace(args[1]))
	if err != nil {
		return fmt.Errorf("id must be a UUID")
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if !cfg.Payments.Enabled() {
		return fmt.Errorf("Nimiq payments are not configured")
	}
	log := logger.New(cfg.Log.Level, cfg.Log.Format).With("service", "recover-payment")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("postgres unavailable")
	}
	defer db.Close()

	client := nimiqRPC.NewClient(cfg.Payments.NimiqRPCURL)
	identities := pgIam.NewNimiqAuthRepo(db)
	verifier := nimiqRPC.NewVerifier(client, cfg.Payments.MerchantAddress, cfg.Payments.NimiqNetwork, identities)

	switch domain {
	case "event-purchase", "event_purchase":
		uc := purchaseUC.NewPurchaseUseCaseWithVerifierAndPolicy(
			pgEventPurchase.NewPurchaseRepo(db),
			cfg.Payments.HoldDuration,
			verifier,
			cfg.Payments.MerchantAddress,
			cfg.Payments.NimiqNetwork,
			purchaseUC.ReconciliationPolicy{
				Interval:  cfg.Payments.ReconciliationInterval,
				Deadline:  cfg.Payments.ReconciliationDeadline,
				BatchSize: cfg.Payments.ReconciliationBatchSize,
			},
		).WithIdentities(identities)
		var userID uuid.UUID
		if err := db.QueryRow(ctx, `SELECT user_id FROM event_purchases WHERE id = $1`, id).Scan(&userID); err != nil {
			return fmt.Errorf("purchase not found")
		}
		result, err := uc.RecoverExpired(ctx, id, userID)
		if err != nil {
			return err
		}
		log.Info("recovery complete", "domain", "event_purchase", "purchase_id", result.Data.ID, "status", result.Data.Status)
		fmt.Printf("event_purchase %s status=%s\n", result.Data.ID, result.Data.Status)
		return nil
	case "payment-request", "payment_request":
		repo := pgPaymentRequest.NewRequestRepo(db)
		request, err := repo.GetByPublicID(ctx, id, time.Now().UTC())
		if err != nil {
			return err
		}
		uc := paymentRequestUC.NewRequestUseCaseWithVerifierAndPolicy(
			repo,
			identities,
			cfg.Payments.RequestTTL,
			cfg.Payments.NimiqNetwork,
			log,
			verifier,
			paymentRequestUC.ReconciliationPolicy{
				Interval:  cfg.Payments.ReconciliationInterval,
				Deadline:  cfg.Payments.ReconciliationDeadline,
				BatchSize: cfg.Payments.ReconciliationBatchSize,
			},
		)
		result, err := uc.RecoverExpired(ctx, id, request.CreatorUserID)
		if err != nil {
			return err
		}
		log.Info("recovery complete", "domain", "payment_request", "public_id", result.Data.PublicID, "status", result.Data.Status)
		fmt.Printf("payment_request %s status=%s\n", result.Data.PublicID, result.Data.Status)
		return nil
	default:
		return fmt.Errorf("usage: recover-payment <event-purchase|payment-request> <id>")
	}
}
