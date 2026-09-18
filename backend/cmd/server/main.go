package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	// Infrastructure
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	apimgmtHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/apimanagement"
	auditHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/audit"
	calendarHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/calendar"
	eventHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/event"
	participationHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/eventparticipation"
	eventpurchaseHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/eventpurchase"
	iamHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/iam"
	paymentrequestHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/paymentrequest"
	placeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/place"
	profileHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/profile"
	realtimeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/realtime"
	tenantHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/tenant"
	walletHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/wallet"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/http/router"
	infraKafka "github.com/masterfabric-go/masterfabric/internal/infrastructure/kafka"
	nimiqRPC "github.com/masterfabric-go/masterfabric/internal/infrastructure/nimiq/rpc"
	pgApimgmt "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/apimanagement"
	pgAudit "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/audit"
	pgCalendar "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/calendar"
	pgEvent "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/event"
	pgParticipation "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/eventparticipation"
	pgEventPurchase "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/eventpurchase"
	pgIam "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/iam"
	pgPaymentRequest "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/paymentrequest"
	pgPlace "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/place"
	pgProfile "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/profile"
	pgTenant "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/tenant"
	infraWS "github.com/masterfabric-go/masterfabric/internal/infrastructure/websocket"

	// Application use cases
	apimgmtUC "github.com/masterfabric-go/masterfabric/internal/application/apimanagement/usecase"
	calendarUC "github.com/masterfabric-go/masterfabric/internal/application/calendar/usecase"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	participationUC "github.com/masterfabric-go/masterfabric/internal/application/eventparticipation/usecase"
	purchaseUC "github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/usecase"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	paymentRequestUC "github.com/masterfabric-go/masterfabric/internal/application/paymentrequest/usecase"
	placeUC "github.com/masterfabric-go/masterfabric/internal/application/place/usecase"
	profileUC "github.com/masterfabric-go/masterfabric/internal/application/profile/usecase"
	realtimeUC "github.com/masterfabric-go/masterfabric/internal/application/realtime/usecase"
	tenantUC "github.com/masterfabric-go/masterfabric/internal/application/tenant/usecase"
	walletUC "github.com/masterfabric-go/masterfabric/internal/application/wallet/usecase"

	// Gateway
	"github.com/masterfabric-go/masterfabric/internal/gateway"
	gatewayInterceptors "github.com/masterfabric-go/masterfabric/internal/infrastructure/gateway/interceptors"

	// Shared
	"github.com/masterfabric-go/masterfabric/internal/shared/cache"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
	"github.com/masterfabric-go/masterfabric/internal/shared/logger"
	"github.com/masterfabric-go/masterfabric/internal/shared/ratelimit"
	"github.com/masterfabric-go/masterfabric/internal/shared/telemetry"
	"github.com/masterfabric-go/masterfabric/internal/shared/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid application configuration: %w", err)
	}

	// Initialize logger
	log := logger.New(cfg.Log.Level, cfg.Log.Format).With("service", version.ServiceName)
	slog.SetDefault(log)

	log.Info("starting NIMNear API",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"email_auth_enabled", cfg.EmailAuth.Enabled,
		"platform_api_enabled", cfg.Platform.APIEnabled,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize OpenTelemetry
	otelShutdown, err := telemetry.Setup(ctx, version.ServiceName, version.Version)
	if err != nil {
		log.Warn("opentelemetry setup failed", "error", err)
	} else {
		defer func() { _ = otelShutdown(context.Background()) }()
		log.Info("opentelemetry initialized")
	}

	// Initialize PostgreSQL connection pool
	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		if cfg.IsProduction() {
			return fmt.Errorf("postgres unavailable in production")
		}
		log.Warn("postgres unavailable, running without database", "reason", "connection failed")
		db = nil
	} else {
		defer db.Close()
		log.Info("connected to postgres")
	}

	// Initialize Redis
	redisClient, err := cache.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		if cfg.IsProduction() {
			return fmt.Errorf("redis unavailable in production")
		}
		log.Warn("redis unavailable, running without cache")
		redisClient = nil
	} else {
		defer redisClient.Close()
		log.Info("connected to redis")
	}

	// Initialize event bus (Kafka or in-process)
	eventBus := initEventBus(ctx, cfg, log)
	defer func() { _ = eventBus.Close() }()

	// Start long-lived background work with a context independent from startup initialization.
	reconciliationCtx, cancelReconciliation := context.WithCancel(context.Background())
	defer cancelReconciliation()

	// Build dependencies
	deps := buildDependencies(reconciliationCtx, log, cfg, db, redisClient, eventBus)

	// Build router
	r := router.New(deps)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-shutdown:
		log.Info("shutdown signal received", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		log.Info("server stopped gracefully")
	}

	return nil
}

// initEventBus creates either a Kafka bus or an in-process bus based on config.
func initEventBus(ctx context.Context, cfg *config.Config, log *slog.Logger) events.EventBus {
	if !cfg.Kafka.Enabled {
		log.Info("using in-process event bus (set KAFKA_ENABLED=true to use Kafka)")
		return events.NewInProcessBus(log, 256)
	}

	log.Info("initializing kafka event bus",
		"brokers", cfg.Kafka.Brokers,
		"group_id", cfg.Kafka.GroupID,
	)

	// Ensure topics exist
	if len(cfg.Kafka.Brokers) > 0 {
		if err := infraKafka.EnsureTopics(
			ctx,
			cfg.Kafka.Brokers[0],
			infraKafka.DefaultTopics(),
			cfg.Kafka.NumPartitions,
			cfg.Kafka.ReplicationFactor,
			log,
		); err != nil {
			log.Warn("failed to ensure kafka topics, falling back to in-process bus", "error", err)
			return events.NewInProcessBus(log, 256)
		}
	}

	kafkaBus := infraKafka.NewBus(cfg.Kafka.Brokers, cfg.Kafka.GroupID, log)

	// Start consuming (after subscriptions are registered in buildDependencies)
	// We start consumption with a background context so it outlives the startup ctx.
	kafkaBus.Start(context.Background())

	log.Info("kafka event bus initialized")
	return kafkaBus
}

func buildDependencies(
	reconciliationCtx context.Context,
	log *slog.Logger,
	cfg *config.Config,
	db *pgxpool.Pool,
	redisClient *redis.Client,
	eventBus events.EventBus,
) router.Dependencies {
	deps := router.Dependencies{
		Logger:             log,
		DB:                 db,
		Redis:              redisClient,
		CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins,
		MaxBodyBytes:       cfg.Server.MaxBodyBytes,
		SessionCookieName:  cfg.NimiqAuth.CookieName,
		PaymentsEnabled:    cfg.Payments.Enabled(),
		EmailAuthEnabled:   cfg.EmailAuth.Enabled,
		PlatformAPIEnabled: cfg.Platform.APIEnabled,
		MetricsEnabled:     cfg.Metrics.Enabled,
		MetricsPublic:      cfg.Metrics.Public,
	}
	if proxies, err := cfg.Server.TrustedProxies(); err == nil {
		deps.TrustedProxies = proxies
	}

	if db == nil {
		log.Warn("database not available, API endpoints will not work")
		return deps
	}

	// --- Repositories ---
	userRepo := pgIam.NewUserRepo(db)
	nimiqAuthRepo := pgIam.NewNimiqAuthRepo(db)
	roleRepo := pgIam.NewRoleRepo(db)
	orgRepo := pgTenant.NewOrgRepo(db)
	workspaceRepo := pgTenant.NewWorkspaceRepository(db)
	appRepo := pgTenant.NewAppRepo(db)
	apiKeyRepo := pgTenant.NewAPIKeyRepo(db)
	endpointRepo := pgApimgmt.NewEndpointRepo(db)
	policyRepo := pgApimgmt.NewPolicyRepo(db)
	auditRepo := pgAudit.NewAuditRepo(db)
	placeRepo := pgPlace.NewPlaceRepo(db)
	calendarRepo := pgCalendar.NewCalendarRepo(db)
	eventRepo := pgEvent.NewEventRepo(db)
	participationRepo := pgParticipation.NewParticipationRepo(db)
	purchaseRepo := pgEventPurchase.NewPurchaseRepo(db)
	paymentRequestRepo := pgPaymentRequest.NewRequestRepo(db)

	// --- Services ---
	jwtService := infraAuth.NewJWTService(cfg.JWT)
	rbacService := infraAuth.NewRBACService(roleRepo, redisClient)

	deps.AuthService = jwtService
	deps.UserRepo = userRepo
	if cfg.Platform.APIEnabled {
		deps.RBACService = rbacService
		deps.OrgRepo = orgRepo
		deps.WorkspaceRepo = workspaceRepo
	}

	// --- Use cases (with event bus for domain event publishing) ---
	registerUC := iamUC.NewRegisterUseCase(userRepo, jwtService, eventBus)
	loginUC := iamUC.NewLoginUseCase(userRepo, jwtService)
	nimiqAuthUC := iamUC.NewNimiqAuthUseCase(nimiqAuthRepo, jwtService, cfg.NimiqAuth)
	assignRoleUC := iamUC.NewAssignRoleUseCase(roleRepo, rbacService, eventBus)
	createOrgUC := tenantUC.NewCreateOrgUseCase(orgRepo, eventBus)
	createWorkspaceUC := tenantUC.NewCreateWorkspaceUseCase(workspaceRepo, orgRepo, eventBus)
	listWorkspacesUC := tenantUC.NewListWorkspacesUseCase(workspaceRepo)
	updateWorkspaceUC := tenantUC.NewUpdateWorkspaceUseCase(workspaceRepo)
	createAppUC := tenantUC.NewCreateAppUseCase(appRepo, orgRepo, eventBus)
	manageKeysUC := tenantUC.NewManageAPIKeysUseCase(apiKeyRepo)
	defineEndpointUC := apimgmtUC.NewDefineEndpointUseCase(endpointRepo, eventBus)
	updatePolicyUC := apimgmtUC.NewUpdatePolicyUseCase(policyRepo)
	retireEndpointUC := apimgmtUC.NewRetireEndpointUseCase(endpointRepo, eventBus)
	activateEndpointUC := apimgmtUC.NewActivateEndpointUseCase(endpointRepo, eventBus)
	eventUseCase := eventUC.NewEventUseCaseWithAssociations(eventRepo, placeRepo, calendarRepo)
	participationUseCase := participationUC.NewParticipationUseCase(participationRepo)
	purchaseUseCase := purchaseUC.NewPurchaseUseCase(purchaseRepo, cfg.Payments.HoldDuration)
	paymentRequestNetwork := strings.TrimSpace(cfg.Payments.NimiqNetwork)
	if paymentRequestNetwork == "" {
		paymentRequestNetwork = cfg.NimiqAuth.Network
	}
	paymentRequestUseCase := paymentRequestUC.NewRequestUseCase(
		paymentRequestRepo,
		nimiqAuthRepo,
		cfg.Payments.RequestTTL,
		paymentRequestNetwork,
		log,
	).WithEventBus(eventBus)
	var nimiqClient *nimiqRPC.Client
	if cfg.Payments.Enabled() {
		nimiqClient = nimiqRPC.NewClient(cfg.Payments.NimiqRPCURL)
		deps.NimiqRPCHealth = nimiqRPC.NewHealth(nimiqClient)
		nimiqVerifier := nimiqRPC.NewVerifier(nimiqClient, cfg.Payments.MerchantAddress, cfg.Payments.NimiqNetwork, nimiqAuthRepo)
		reconciliationPolicy := purchaseUC.ReconciliationPolicy{
			Interval:  cfg.Payments.ReconciliationInterval,
			Deadline:  cfg.Payments.ReconciliationDeadline,
			BatchSize: cfg.Payments.ReconciliationBatchSize,
		}
		purchaseUseCase = purchaseUC.NewPurchaseUseCaseWithVerifierAndPolicy(
			purchaseRepo,
			cfg.Payments.HoldDuration,
			nimiqVerifier,
			cfg.Payments.MerchantAddress,
			cfg.Payments.NimiqNetwork,
			reconciliationPolicy,
		).WithIdentities(nimiqAuthRepo)
		purchaseWorker := purchaseUC.NewReconciliationWorker(purchaseUseCase, log)
		go purchaseWorker.Run(reconciliationCtx)
		paymentRequestUseCase = paymentRequestUC.NewRequestUseCaseWithVerifierAndPolicy(
			paymentRequestRepo,
			nimiqAuthRepo,
			cfg.Payments.RequestTTL,
			cfg.Payments.NimiqNetwork,
			log,
			nimiqVerifier,
			paymentRequestUC.ReconciliationPolicy{
				Interval:  cfg.Payments.ReconciliationInterval,
				Deadline:  cfg.Payments.ReconciliationDeadline,
				BatchSize: cfg.Payments.ReconciliationBatchSize,
			},
		)
		paymentRequestWorker := paymentRequestUC.NewReconciliationWorker(paymentRequestUseCase, log)
		go paymentRequestWorker.Run(reconciliationCtx)
	}
	purchaseUseCase = purchaseUseCase.WithEventBus(eventBus)
	paymentRequestUseCase = paymentRequestUseCase.WithEventBus(eventBus)
	walletNetwork := strings.TrimSpace(cfg.Payments.NimiqNetwork)
	if walletNetwork == "" {
		walletNetwork = cfg.NimiqAuth.Network
	}
	walletUseCase := walletUC.NewWalletUseCase(nimiqAuthRepo, nimiqRPC.NewWalletReader(nimiqClient), walletNetwork)
	profileUseCase := profileUC.NewProfileUseCase(pgProfile.NewProfileRepo(db), eventRepo)
	nearbyPlacesUC := placeUC.NewNearbyPlacesUseCase(placeRepo)
	calendarUseCase := calendarUC.NewCalendarUseCase(calendarRepo, eventRepo)

	// --- Register Kafka consumers ---
	// Log all IAM events
	eventBus.Subscribe(events.TopicIAM, func(ctx context.Context, event events.Event) error {
		log.Info("iam event received", "event", event)
		return nil
	})
	// Log all tenant events
	eventBus.Subscribe(events.TopicTenant, func(ctx context.Context, event events.Event) error {
		log.Info("tenant event received", "event", event)
		return nil
	})
	// Log all API management events
	eventBus.Subscribe(events.TopicAPIManagement, func(ctx context.Context, event events.Event) error {
		log.Info("api-management event received", "event", event)
		return nil
	})

	// --- Handlers ---
	deps.IAMHandler = iamHandler.NewHandler(registerUC, loginUC, assignRoleUC, userRepo, nimiqAuthUC, cfg.NimiqAuth, time.Duration(cfg.JWT.ExpirationHours)*time.Hour, authLimiter(redisClient))
	deps.IAMHandler.SetEmailAuthEnabled(cfg.EmailAuth.Enabled)
	deps.IAMHandler.SetDeleteAccountUseCase(iamUC.NewDeleteAccountUseCase(pgIam.NewAccountDeletionRepo(db)))
	if cfg.Platform.APIEnabled {
		deps.TenantHandler = tenantHandler.NewHandler(
			createOrgUC,
			createAppUC,
			manageKeysUC,
			createWorkspaceUC,
			listWorkspacesUC,
			updateWorkspaceUC,
			orgRepo,
			appRepo,
		)
		deps.APIMgmtHandler = apimgmtHandler.NewHandler(defineEndpointUC, updatePolicyUC, retireEndpointUC, activateEndpointUC, endpointRepo, policyRepo)
		deps.AuditHandler = auditHandler.NewHandler(auditRepo)
	}
	deps.EventHandler = eventHandler.NewHandler(eventUseCase)
	deps.ParticipationHandler = participationHandler.NewHandler(participationUseCase)
	deps.PurchaseHandler = eventpurchaseHandler.NewHandler(purchaseUseCase, authLimiter(redisClient), eventpurchaseHandler.Limits{
		Create: cfg.Payments.PurchaseCreateLimit,
		Submit: cfg.Payments.PurchaseSubmitLimit,
		Window: cfg.Payments.PurchaseRateLimitWindow,
	})
	deps.PaymentRequestHandler = paymentrequestHandler.NewHandler(paymentRequestUseCase, authLimiter(redisClient), paymentrequestHandler.Limits{
		Create: cfg.Payments.RequestCreateLimit,
		Lookup: cfg.Payments.RequestLookupLimit,
		Submit: cfg.Payments.RequestSubmitLimit,
		Window: cfg.Payments.RequestRateLimitWindow,
	})
	deps.WalletHandler = walletHandler.NewHandler(walletUseCase)
	deps.ProfileHandler = profileHandler.NewHandler(profileUseCase)
	deps.PlaceHandler = placeHandler.NewHandler(nearbyPlacesUC)
	deps.CalendarHandler = calendarHandler.NewHandler(calendarUseCase)

	// --- WebSocket real-time hub ---
	wsHub := infraWS.NewHub(log, cfg.WebSocket.MaxConnections)
	eventBridge := infraWS.NewEventBridge(wsHub, appRepo, log)
	eventBridge.Register(eventBus)
	instanceID := uuid.New().String()
	if redisRelay := infraWS.NewRedisRelay(redisClient, instanceID, log); redisRelay != nil {
		eventBridge.SetRelay(redisRelay, instanceID)
		go redisRelay.Listen(reconciliationCtx, eventBridge.DeliverFanout)
		log.Info("realtime redis fanout enabled")
	} else {
		log.Info("realtime redis fanout disabled, local websocket delivery only")
	}

	validateConnectUC := realtimeUC.NewValidateConnectUseCase(appRepo, rbacService).WithUsers(userRepo)
	wsUpgrader := infraWS.NewUpgrader(infraWS.UpgraderConfig{
		ReadBufferSize:   cfg.WebSocket.ReadBufferSize,
		WriteBufferSize:  cfg.WebSocket.WriteBufferSize,
		AllowedOrigins:   cfg.Server.CORSAllowedOrigins,
		AllowEmptyOrigin: cfg.WebSocket.AllowEmptyOrigin,
	})
	deps.RealtimeHandler = realtimeHandler.NewHandler(realtimeHandler.Config{
		ValidateUC:   validateConnectUC,
		AuthService:  jwtService,
		Hub:          wsHub,
		Upgrader:     wsUpgrader,
		PingInterval: cfg.WebSocket.PingIntervalSec,
		Logger:       log,
		Enabled:      cfg.WebSocket.Enabled,
		CookieName:   cfg.NimiqAuth.CookieName,
	})

	// --- Gateway pipeline with interceptors ---
	// Create interceptor chain: schema validation, PII masking, request/response transformers
	piiMasker := gatewayInterceptors.NewPIIMasker(
		[]string{"password", "password_hash", "api_key", "secret", "token", "ssn", "credit_card"},
		"***",
	)
	schemaValidator := gatewayInterceptors.NewSchemaValidator()

	// Create dynamic handler resolver for routing requests to backend service handlers
	// This supports:
	// 1. Registered handlers (if you register specific handlers)
	// 2. HTTP proxy to external services (if backend_service is a URL or configured)
	// 3. Generic dynamic database handler (automatically performs CRUD operations)
	backendRegistry := gateway.NewBackendRegistry()
	dynamicResolver := gateway.NewDynamicHandlerResolver(backendRegistry, log, db).WithAllowedTables(cfg.Platform.GatewayTableAllowlist)

	// Service proxies and custom handlers must be backed by an authoritative
	// data source and registered explicitly. NIMNear registers none by default.

	// Wire interceptors into gateway pipeline with dynamic resolver
	if cfg.Platform.APIEnabled {
		deps.GatewayPipeline = gateway.NewPipeline(
			endpointRepo,
			policyRepo,
			rbacService,
			redisClient,
			log,
			dynamicResolver,
			schemaValidator,
			piiMasker,
		)
	}

	return deps
}

func authLimiter(redisClient *redis.Client) ratelimit.Limiter {
	if limiter := ratelimit.NewRedisLimiter(redisClient); limiter != nil {
		return limiter
	}
	return ratelimit.NewMemoryLimiter()
}
