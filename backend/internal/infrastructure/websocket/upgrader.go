package websocket

import (
	"net/http"
	"strings"
	"time"

	gorillaws "github.com/gorilla/websocket"
)

const (
	defaultReadBufferSize  = 1024
	defaultWriteBufferSize = 1024
	defaultPingInterval    = 30 * time.Second
)

// UpgraderConfig holds WebSocket upgrader settings.
type UpgraderConfig struct {
	ReadBufferSize   int
	WriteBufferSize  int
	AllowedOrigins   []string
	AllowEmptyOrigin bool
}

// NewUpgrader creates a configured gorilla/websocket upgrader.
func NewUpgrader(cfg UpgraderConfig) gorillaws.Upgrader {
	readBuf := cfg.ReadBufferSize
	if readBuf <= 0 {
		readBuf = defaultReadBufferSize
	}
	writeBuf := cfg.WriteBufferSize
	if writeBuf <= 0 {
		writeBuf = defaultWriteBufferSize
	}

	return gorillaws.Upgrader{
		ReadBufferSize:  readBuf,
		WriteBufferSize: writeBuf,
		CheckOrigin:     originChecker(cfg.AllowedOrigins, cfg.AllowEmptyOrigin),
	}
}

func originChecker(allowed []string, allowEmpty bool) func(*http.Request) bool {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowedSet[trimmed] = struct{}{}
		}
	}
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return allowEmpty
		}
		if _, ok := allowedSet["*"]; ok {
			return true
		}
		if len(allowedSet) == 0 {
			return allowEmpty
		}
		_, ok := allowedSet[origin]
		return ok
	}
}

// PingInterval returns the configured ping interval.
func PingInterval(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultPingInterval
	}
	return time.Duration(seconds) * time.Second
}
