package router

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	// Handlers
	apimgmtHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/apimanagement"
	auditHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/audit"
	calendarHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/calendar"
	eventHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/event"
	participationHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/eventparticipation"
	eventpurchaseHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/eventpurchase"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/health"
	iamHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/iam"
	placeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/place"
	profileHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/profile"
	realtimeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/realtime"
	tenantHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/tenant"

	// Services & middleware
	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/gateway"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"

	// Repositories (for tenant resolver middleware)
	tenantRepo "github.com/masterfabric-go/masterfabric/internal/domain/tenant/repository"
)

func maybeRequirePermission(rbac iamService.RBACService, permission string) func(http.Handler) http.Handler {
	if rbac == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.RequirePermission(rbac, permission)
}

// Dependencies holds all injected dependencies for the router.
type Dependencies struct {
	Logger *slog.Logger
	DB     *pgxpool.Pool
	Redis  *redis.Client

	CORSAllowedOrigins []string
	MaxBodyBytes       int64

	// Services
	AuthService iamService.AuthService
	RBACService iamService.RBACService

	// Handlers
	IAMHandler           *iamHandler.Handler
	TenantHandler        *tenantHandler.Handler
	APIMgmtHandler       *apimgmtHandler.Handler
	AuditHandler         *auditHandler.Handler
	EventHandler         *eventHandler.Handler
	ParticipationHandler *participationHandler.Handler
	PurchaseHandler      *eventpurchaseHandler.Handler
	ProfileHandler       *profileHandler.Handler
	PlaceHandler         *placeHandler.Handler
	CalendarHandler      *calendarHandler.Handler
	RealtimeHandler      *realtimeHandler.Handler

	// Gateway
	GatewayPipeline *gateway.Pipeline

	// Repos needed for middleware
	OrgRepo       tenantRepo.OrgRepository
	WorkspaceRepo tenantRepo.WorkspaceRepository
}

// New creates the root Chi router with all middleware and routes.
func New(deps Dependencies) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(deps.Logger))
	r.Use(middleware.Recoverer(deps.Logger))
	if deps.MaxBodyBytes > 0 {
		r.Use(middleware.MaxBodyBytes(deps.MaxBodyBytes))
	}
	r.Use(cors.Handler(middleware.CORSOptions(deps.CORSAllowedOrigins)))

	// Health endpoints
	healthHandler := health.NewHandler(deps.DB, deps.Redis)
	r.Get("/health/live", healthHandler.Liveness)
	r.Get("/health/ready", healthHandler.Readiness)

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes (no JWT required)
		r.Route("/auth", func(r chi.Router) {
			if deps.IAMHandler != nil {
				r.Post("/register", deps.IAMHandler.Register)
				r.Post("/login", deps.IAMHandler.Login)
			}
		})

		// Public place discovery does not require authentication or tenant context.
		if deps.PlaceHandler != nil {
			r.Get("/places/nearby", deps.PlaceHandler.ListNearby)
			r.Get("/places/{id}", deps.PlaceHandler.Get)
		}
		if deps.EventHandler != nil {
			r.Get("/events", deps.EventHandler.List)
			r.Get("/events/{id}", deps.EventHandler.Get)
		}
		if deps.CalendarHandler != nil {
			r.Get("/calendars", deps.CalendarHandler.ListPublic)
			r.Get("/calendars/{id}", deps.CalendarHandler.GetPublic)
		}
		if deps.ProfileHandler != nil {
			r.Get("/profiles/{id}", deps.ProfileHandler.GetPublic)
			r.Get("/profiles/{id}/events", deps.ProfileHandler.ListEvents)
		}

		// Protected routes (require JWT)
		r.Group(func(r chi.Router) {
			if deps.AuthService != nil {
				r.Use(middleware.JWTAuth(deps.AuthService))
			}

			// Tenant resolution middleware (with workspace support)
			if deps.OrgRepo != nil {
				// Note: WorkspaceRepo can be nil - workspace resolution is optional
				r.Use(middleware.TenantResolverWithWorkspace(deps.OrgRepo, deps.WorkspaceRepo))
			}

			// WebSocket endpoint (before gateway pipeline — upgrade requests are not HTTP proxy)
			if deps.RealtimeHandler != nil {
				r.Get("/ws", deps.RealtimeHandler.Connect)
			}

			// Gateway pipeline (rate limiting, permission enforcement for managed endpoints).
			// Chi requires every middleware on a mux to be registered before any route on
			// that mux, so the pipeline-scoped routes live in their own group. It inherits
			// JWT auth + tenant resolution from the parent group, while /ws above stays
			// outside the pipeline.
			r.Group(func(r chi.Router) {
				if deps.GatewayPipeline != nil {
					r.Use(deps.GatewayPipeline.Enforce)
				}

				// User routes
				if deps.IAMHandler != nil {
					r.Get("/me", deps.IAMHandler.GetMe)
					if deps.ProfileHandler != nil {
						r.Patch("/me/profile", deps.ProfileHandler.UpdateMe)
					}
					r.With(maybeRequirePermission(deps.RBACService, "user:read")).Route("/users", func(r chi.Router) {
						r.Get("/", deps.IAMHandler.ListUsers)
						r.Get("/{id}", deps.IAMHandler.GetUser)
					})
					r.With(maybeRequirePermission(deps.RBACService, "user:write")).Post("/roles/assign", deps.IAMHandler.AssignRole)
				}

				if deps.EventHandler != nil {
					r.Post("/events", deps.EventHandler.Create)
				}

				if deps.CalendarHandler != nil {
					r.Get("/me/calendars", deps.CalendarHandler.ListMine)
					r.Post("/calendars", deps.CalendarHandler.Create)
					r.Post("/calendars/{id}/follow", deps.CalendarHandler.Follow)
					r.Delete("/calendars/{id}/follow", deps.CalendarHandler.Unfollow)
				}

				if deps.ParticipationHandler != nil {
					r.Get("/events/{id}/rsvp", deps.ParticipationHandler.GetState)
					r.Post("/events/{id}/rsvp", deps.ParticipationHandler.RSVP)
					r.Delete("/events/{id}/rsvp", deps.ParticipationHandler.Cancel)
				}

				if deps.PurchaseHandler != nil {
					r.Get("/events/{id}/purchases/current", deps.PurchaseHandler.GetCurrent)
					r.Post("/events/{id}/purchases", deps.PurchaseHandler.Create)
					r.Get("/purchases/{id}", deps.PurchaseHandler.Get)
					r.Get("/purchases/{id}/payment-instructions", deps.PurchaseHandler.PaymentInstructions)
					r.Post("/purchases/{id}/transaction", deps.PurchaseHandler.SubmitTransaction)
				}

				// Organization routes
				if deps.TenantHandler != nil {
					r.Route("/organizations", func(r chi.Router) {
						r.With(maybeRequirePermission(deps.RBACService, "org:write")).Post("/", deps.TenantHandler.CreateOrg)
						r.With(maybeRequirePermission(deps.RBACService, "org:read")).Get("/", deps.TenantHandler.ListOrgs)
						r.Route("/{orgId}", func(r chi.Router) {
							r.With(maybeRequirePermission(deps.RBACService, "org:read")).Get("/", deps.TenantHandler.GetOrg)

							// Apps under organization
							r.Route("/apps", func(r chi.Router) {
								r.With(maybeRequirePermission(deps.RBACService, "app:write")).Post("/", deps.TenantHandler.CreateApp)
								r.With(maybeRequirePermission(deps.RBACService, "app:read")).Get("/", deps.TenantHandler.ListApps)
								r.Route("/{appId}", func(r chi.Router) {
									r.With(maybeRequirePermission(deps.RBACService, "app:read")).Get("/", deps.TenantHandler.GetApp)

									// API keys under app
									r.Route("/keys", func(r chi.Router) {
										r.With(maybeRequirePermission(deps.RBACService, "app:write")).Post("/", deps.TenantHandler.CreateAPIKey)
										r.With(maybeRequirePermission(deps.RBACService, "app:read")).Get("/", deps.TenantHandler.ListAPIKeys)
										r.With(maybeRequirePermission(deps.RBACService, "app:write")).Delete("/{keyId}", deps.TenantHandler.RevokeAPIKey)
									})

									// Endpoints under app
									if deps.APIMgmtHandler != nil {
										r.Route("/endpoints", func(r chi.Router) {
											r.With(maybeRequirePermission(deps.RBACService, "endpoint:write")).Post("/", deps.APIMgmtHandler.DefineEndpoint)
											r.With(maybeRequirePermission(deps.RBACService, "endpoint:read")).Get("/", deps.APIMgmtHandler.ListEndpoints)
											r.Route("/{endpointId}", func(r chi.Router) {
												r.With(maybeRequirePermission(deps.RBACService, "endpoint:read")).Get("/", deps.APIMgmtHandler.GetEndpoint)
												r.With(maybeRequirePermission(deps.RBACService, "endpoint:write")).Post("/retire", deps.APIMgmtHandler.RetireEndpoint)
												r.With(maybeRequirePermission(deps.RBACService, "endpoint:write")).Post("/activate", deps.APIMgmtHandler.ActivateEndpoint)
												r.With(maybeRequirePermission(deps.RBACService, "endpoint:write")).Put("/policy", deps.APIMgmtHandler.UpdatePolicy)
												r.With(maybeRequirePermission(deps.RBACService, "endpoint:read")).Get("/policy", deps.APIMgmtHandler.GetPolicy)
											})
										})
									}
								})
							})

							// Workspaces under organization
							r.Route("/workspaces", func(r chi.Router) {
								r.With(maybeRequirePermission(deps.RBACService, "org:write")).Post("/", deps.TenantHandler.CreateWorkspace)
								r.With(maybeRequirePermission(deps.RBACService, "org:read")).Get("/", deps.TenantHandler.ListWorkspaces)
								r.Route("/{workspaceId}", func(r chi.Router) {
									r.With(maybeRequirePermission(deps.RBACService, "org:write")).Put("/", deps.TenantHandler.UpdateWorkspace)
								})
							})

							// Audit logs under organization
							if deps.AuditHandler != nil {
								r.With(maybeRequirePermission(deps.RBACService, "org:read")).Get("/audit-logs", deps.AuditHandler.ListByOrg)
							}
						})
					})
				}

				// Audit logs by user
				if deps.AuditHandler != nil {
					r.With(maybeRequirePermission(deps.RBACService, "org:read")).Get("/users/{userId}/audit-logs", deps.AuditHandler.ListByUser)
				}

				// Catch-all handler for managed endpoints (must be last in the group)
				// This allows the gateway pipeline to handle dynamic endpoints like /api/v1/products
				// The gateway middleware will validate and return responses for managed endpoints
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Gateway middleware should have already handled this if it's a managed endpoint
					// If we reach here, it means no endpoint was found, return 404
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"error":"endpoint not found","code":404,"message":"No endpoint registered for this path. Define the endpoint first using POST /api/v1/organizations/{orgId}/apps/{appId}/endpoints"}`))
				})
			})
		})
	})

	// Catch-all handler for managed endpoints (must be after all specific routes)
	// This allows the gateway pipeline to handle dynamic endpoints like /api/v1/products
	// The gateway middleware will validate and return responses for managed endpoints
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		// If this is an API v1 path, let the gateway handle it (if it hasn't already)
		// Otherwise return 404
		if !strings.HasPrefix(r.URL.Path, "/api/v1") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found","code":404}`))
			return
		}

		// For /api/v1 paths, check if gateway pipeline already handled it
		// If not, return 404 (gateway would have returned response if endpoint existed)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"endpoint not found","code":404,"message":"No endpoint registered for this path. Define the endpoint first."}`))
	})

	return r
}
