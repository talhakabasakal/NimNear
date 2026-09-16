package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/masterfabric-go/masterfabric/internal/gateway"
	apimgmtHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/apimanagement"
	auditHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/audit"
	calendarHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/calendar"
	eventHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/event"
	participationHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/eventparticipation"
	iamHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/iam"
	placeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/place"
	profileHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/profile"
	realtimeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/realtime"
	tenantHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/tenant"
)

// fullyWiredDeps returns Dependencies with every optional handler and the
// gateway pipeline present, which is the wiring that exercises every route
// and middleware registration in New.
func fullyWiredDeps() Dependencies {
	return Dependencies{
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
		MaxBodyBytes:         1 << 20,
		IAMHandler:           &iamHandler.Handler{},
		TenantHandler:        &tenantHandler.Handler{},
		APIMgmtHandler:       &apimgmtHandler.Handler{},
		AuditHandler:         &auditHandler.Handler{},
		CalendarHandler:      &calendarHandler.Handler{},
		EventHandler:         &eventHandler.Handler{},
		ParticipationHandler: &participationHandler.Handler{},
		ProfileHandler:       &profileHandler.Handler{},
		PlaceHandler:         &placeHandler.Handler{},
		RealtimeHandler:      &realtimeHandler.Handler{},
		GatewayPipeline:      &gateway.Pipeline{},
	}
}

// TestNewRegistersMiddlewareBeforeRoutes guards against the chi constraint that
// every middleware on a mux must be registered before any route on that mux.
// Violating it makes New panic with
// "chi: all middlewares must be defined before routes on a mux", which only
// surfaces at server start-up.
func TestNewRegistersMiddlewareBeforeRoutes(t *testing.T) {
	t.Run("fully wired", func(t *testing.T) {
		if r := New(fullyWiredDeps()); r == nil {
			t.Fatal("New returned nil router")
		}
	})

	t.Run("no optional dependencies", func(t *testing.T) {
		if r := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}); r == nil {
			t.Fatal("New returned nil router")
		}
	})
}

// TestNewRoutePatterns pins the externally reachable route patterns so a future
// regrouping of middleware cannot silently drop or relocate an endpoint.
func TestNewRoutePatterns(t *testing.T) {
	r := New(fullyWiredDeps())

	registered := map[string]bool{}
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	want := []string{
		"GET /health/live",
		"GET /health/ready",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/places/nearby",
		"GET /api/v1/places/{id}",
		"GET /api/v1/events",
		"GET /api/v1/events/{id}",
		"GET /api/v1/calendars",
		"GET /api/v1/calendars/{id}",
		"GET /api/v1/profiles/{id}",
		"GET /api/v1/profiles/{id}/events",
		"POST /api/v1/events",
		"GET /api/v1/me/calendars",
		"POST /api/v1/calendars",
		"POST /api/v1/calendars/{id}/follow",
		"DELETE /api/v1/calendars/{id}/follow",
		"GET /api/v1/events/{id}/rsvp",
		"POST /api/v1/events/{id}/rsvp",
		"DELETE /api/v1/events/{id}/rsvp",
		"GET /api/v1/ws",
		"GET /api/v1/me",
		"PATCH /api/v1/me/profile",
		"POST /api/v1/organizations/",
		"GET /api/v1/organizations/{orgId}/apps/{appId}/endpoints/",
		"GET /api/v1/organizations/{orgId}/workspaces/",
		"GET /api/v1/organizations/{orgId}/audit-logs",
	}
	for _, w := range want {
		if !registered[w] {
			got := make([]string, 0, len(registered))
			for k := range registered {
				got = append(got, k)
			}
			sort.Strings(got)
			t.Errorf("route %q not registered; registered routes:\n  %v", w, got)
		}
	}
}

// TestSampleProductRouteIsNotReachable guards against reintroducing the removed
// sample handler or a fixed product route into the NIMNear application.
func TestSampleProductRouteIsNotReachable(t *testing.T) {
	r := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	recorder := httptest.NewRecorder()

	r.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/products status = %d, want 404", recorder.Code)
	}
	body := recorder.Body.String()
	for _, fabricated := range []string{"Product 1", "Product 2", "new-id"} {
		if strings.Contains(body, fabricated) {
			t.Fatalf("sample record %q was reachable: %s", fabricated, body)
		}
	}
}
