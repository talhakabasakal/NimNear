// Package realtime provides the WebSocket HTTP handler for real-time event delivery.
package realtime

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	realtimeUC "github.com/masterfabric-go/masterfabric/internal/application/realtime/usecase"
	iamService "github.com/masterfabric-go/masterfabric/internal/domain/iam/service"
	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
	realtimeService "github.com/masterfabric-go/masterfabric/internal/domain/realtime/service"
	infraWS "github.com/masterfabric-go/masterfabric/internal/infrastructure/websocket"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
	"github.com/masterfabric-go/masterfabric/internal/shared/response"
)

// Handler handles WebSocket upgrade requests.
type Handler struct {
	validateUC *realtimeUC.ValidateConnectUseCase
	auth       iamService.AuthService
	hub        *infraWS.Hub
	upgrader   gorillaws.Upgrader
	pingSecs   int
	logger     *slog.Logger
	enabled    bool
	cookieName string
}

// Config holds handler dependencies.
type Config struct {
	ValidateUC   *realtimeUC.ValidateConnectUseCase
	AuthService  iamService.AuthService
	Hub          *infraWS.Hub
	Upgrader     gorillaws.Upgrader
	PingInterval int
	Logger       *slog.Logger
	Enabled      bool
	CookieName   string
}

// NewHandler creates a new WebSocket handler.
func NewHandler(cfg Config) *Handler {
	return &Handler{
		validateUC: cfg.ValidateUC,
		auth:       cfg.AuthService,
		hub:        cfg.Hub,
		upgrader:   cfg.Upgrader,
		pingSecs:   cfg.PingInterval,
		logger:     cfg.Logger,
		enabled:    cfg.Enabled,
		cookieName: cfg.CookieName,
	}
}

// Connect upgrades the HTTP connection to WebSocket and registers the client.
func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		response.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "websocket is disabled"})
		return
	}

	userID, orgClaim, err := h.resolveUser(r)
	if err != nil {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	orgID := resolveOrgID(r, orgClaim)
	appIDStr := strings.TrimSpace(r.Header.Get("X-App-ID"))
	var input *realtimeUC.ConnectInput
	if appIDStr == "" {
		if h.validateUC == nil {
			input = &realtimeUC.ConnectInput{UserID: userID}
		} else {
			input, err = h.validateUC.ExecuteUser(r.Context(), userID)
		}
		if err != nil {
			response.Error(w, err)
			return
		}
	} else {
		appID, parseErr := realtimeUC.ParseAppHeader(appIDStr)
		if parseErr != nil {
			response.Error(w, parseErr)
			return
		}
		if h.validateUC == nil {
			response.JSON(w, http.StatusBadRequest, map[string]string{"error": "app context required"})
			return
		}
		input, err = h.validateUC.Execute(r.Context(), userID, orgID, appID)
		if err != nil {
			response.Error(w, err)
			return
		}
	}
	if input == nil || input.UserID == uuid.Nil {
		response.JSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authentication session"})
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("websocket upgrade failed", "error", err)
		}
		return
	}

	clientID := infraWS.NewClientID()
	send := make(chan []byte, 64)
	info := realtimeService.ClientInfo{
		ID:             clientID,
		UserID:         input.UserID,
		OrganizationID: input.OrganizationID,
		AppID:          input.AppID,
	}

	unregister := h.hub.Register(info, send)

	wsClient := &infraWS.Session{
		ID:       clientID,
		Conn:     conn,
		Send:     send,
		Hub:      h.hub,
		OnAction: h.handleClientAction,
	}
	wsClient.Start(infraWS.PingInterval(h.pingSecs), unregister)

	if h.logger != nil {
		h.logger.Info("websocket client connected",
			"client_id", clientID,
			"user_id", input.UserID,
			"org_id", input.OrganizationID,
			"app_id", input.AppID,
			"scope", connectionScope(input),
		)
	}

	// Send welcome subscribed message for default channel.
	welcome, _ := json.Marshal(model.NewControlMessage(model.TypeSubscribed, model.DefaultChannel, "connected"))
	select {
	case send <- welcome:
	default:
	}
}

func (h *Handler) handleClientAction(clientID string, raw []byte) {
	var msg model.InboundMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		h.sendError(clientID, "invalid message format")
		return
	}

	switch msg.Action {
	case model.ActionPing:
		h.sendControl(clientID, model.TypePong, "", "")
	case model.ActionSubscribe:
		if err := model.ValidateChannelName(msg.Channel); err != nil {
			h.sendError(clientID, err.Error())
			return
		}
		if err := h.hub.Subscribe(clientID, msg.Channel); err != nil {
			h.sendError(clientID, err.Error())
			return
		}
		h.sendControl(clientID, model.TypeSubscribed, msg.Channel, "")
	case model.ActionUnsubscribe:
		if err := model.ValidateChannelName(msg.Channel); err != nil {
			h.sendError(clientID, err.Error())
			return
		}
		if err := h.hub.Unsubscribe(clientID, msg.Channel); err != nil {
			h.sendError(clientID, err.Error())
			return
		}
	default:
		h.sendError(clientID, "unknown action")
	}
}

func (h *Handler) sendControl(clientID, msgType, channel, message string) {
	payload, _ := json.Marshal(model.NewControlMessage(msgType, channel, message))
	h.hub.SendToClient(clientID, payload)
}

func (h *Handler) sendError(clientID, message string) {
	h.sendControl(clientID, model.TypeError, "", message)
}

func (h *Handler) resolveUser(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	if id, ok := middleware.UserIDFromContext(r.Context()); ok && id != uuid.Nil {
		orgID, _ := middleware.OrgIDFromContext(r.Context())
		return id, orgID, nil
	}
	token := h.extractToken(r)
	if token == "" {
		return uuid.Nil, uuid.Nil, errors.New("missing authentication session")
	}
	if h.auth == nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid token")
	}
	claims, err := h.auth.ValidateToken(r.Context(), token)
	if err != nil || claims == nil || claims.UserID == uuid.Nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid token")
	}
	return claims.UserID, claims.OrganizationID, nil
}

func (h *Handler) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	cookieName := h.cookieName
	if cookieName == "" {
		cookieName = "nimnear_session"
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		if token := strings.TrimSpace(cookie.Value); token != "" {
			return token
		}
	}
	return ""
}

func connectionScope(input *realtimeUC.ConnectInput) string {
	if input != nil && input.AppID != uuid.Nil && input.OrganizationID != uuid.Nil {
		return "app"
	}
	return "user"
}

func resolveOrgID(r *http.Request, claimOrgID uuid.UUID) uuid.UUID {
	if header := r.Header.Get("X-Organization-ID"); header != "" {
		if parsed, err := uuid.Parse(header); err == nil {
			return parsed
		}
	}
	if claimOrgID != uuid.Nil {
		return claimOrgID
	}
	if tenantOrg, ok := middleware.TenantIDFromContext(r.Context()); ok {
		return tenantOrg
	}
	return uuid.Nil
}
