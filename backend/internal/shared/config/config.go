package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvironmentDevelopment = "development"
	EnvironmentTest        = "test"
	EnvironmentProduction  = "production"

	defaultJWTSecret  = "change-me-in-production"
	defaultJWTIssuer  = "masterfabric"
	defaultDBHost     = "localhost"
	defaultDBUser     = "masterfabric"
	defaultDBPassword = "masterfabric"
	defaultDBName     = "masterfabric"
	defaultDBSSLMode  = "disable"
)

// Config holds all application configuration.
type Config struct {
	Environment string
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	JWT         JWTConfig
	Kafka       KafkaConfig
	WebSocket   WebSocketConfig
	Payments    PaymentConfig
	Log         LogConfig
}

// IsProduction reports whether the configuration targets a production environment.
func (c Config) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.Environment), EnvironmentProduction)
}

// Validate checks environment-sensitive configuration before any infrastructure
// or server resources are initialized. Errors intentionally contain variable
// names and remediation guidance only; secret values are never included.
func (c Config) Validate() error {
	environment := strings.ToLower(strings.TrimSpace(c.Environment))
	switch environment {
	case EnvironmentDevelopment, EnvironmentTest:
	case EnvironmentProduction:
		if err := validateProductionJWT(c.JWT); err != nil {
			return err
		}
		if err := validateProductionDatabase(c.Database); err != nil {
			return err
		}
		if len(c.Server.CORSAllowedOrigins) == 0 || containsWildcardOrigin(c.Server.CORSAllowedOrigins) {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must contain explicit origins in production")
		}
	default:
		return fmt.Errorf("APP_ENV must be one of development, test, or production")
	}

	if err := c.Payments.ValidateForEnvironment(environment); err != nil {
		return err
	}
	return nil
}

func validateProductionJWT(cfg JWTConfig) error {
	secret := strings.TrimSpace(cfg.Secret)
	if secret == "" || secret == defaultJWTSecret {
		return fmt.Errorf("JWT_SECRET must be explicitly configured in production")
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must contain at least 32 characters in production")
	}
	if strings.TrimSpace(cfg.Issuer) == "" || strings.EqualFold(strings.TrimSpace(cfg.Issuer), defaultJWTIssuer) {
		return fmt.Errorf("JWT_ISSUER must be explicitly configured in production")
	}
	return nil
}

func validateProductionDatabase(cfg DatabaseConfig) error {
	if strings.TrimSpace(cfg.Host) == "" || strings.EqualFold(strings.TrimSpace(cfg.Host), defaultDBHost) {
		return fmt.Errorf("DB_HOST must be explicitly configured in production")
	}
	if strings.TrimSpace(cfg.User) == "" || strings.EqualFold(strings.TrimSpace(cfg.User), defaultDBUser) {
		return fmt.Errorf("DB_USER must be explicitly configured in production")
	}
	if strings.TrimSpace(cfg.Password) == "" || cfg.Password == defaultDBPassword {
		return fmt.Errorf("DB_PASSWORD must be explicitly configured in production")
	}
	if strings.TrimSpace(cfg.DBName) == "" || strings.EqualFold(strings.TrimSpace(cfg.DBName), defaultDBName) {
		return fmt.Errorf("DB_NAME must be explicitly configured in production")
	}
	if strings.EqualFold(strings.TrimSpace(cfg.SSLMode), defaultDBSSLMode) {
		return fmt.Errorf("DB_SSLMODE must require TLS in production")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("DB_PORT must be a valid TCP port in production")
	}
	return nil
}

func containsWildcardOrigin(origins []string) bool {
	for _, origin := range origins {
		if strings.TrimSpace(origin) == "*" {
			return true
		}
	}
	return false
}

// WebSocketConfig holds real-time WebSocket settings.
type WebSocketConfig struct {
	Enabled         bool
	MaxConnections  int
	PingIntervalSec int
	ReadBufferSize  int
	WriteBufferSize int
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host               string
	Port               int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	CORSAllowedOrigins []string
	MaxBodyBytes       int64
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

// DSN returns the PostgreSQL connection string with escaped credentials.
func (d DatabaseConfig) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   fmt.Sprintf("%s:%d", d.Host, d.Port),
		Path:   "/" + d.DBName,
	}
	u.RawQuery = url.Values{"sslmode": {d.SSLMode}}.Encode()
	return u.String()
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// Addr returns the Redis address string.
func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// JWTConfig holds JWT signing settings.
type JWTConfig struct {
	Secret          string
	ExpirationHours int
	Issuer          string
}

// KafkaConfig holds Kafka connection and consumer settings.
type KafkaConfig struct {
	Brokers           []string
	GroupID           string
	Enabled           bool
	NumPartitions     int
	ReplicationFactor int
}

// PaymentConfig contains public payment verification settings. No secret or private key is accepted.
type PaymentConfig struct {
	HoldDuration    time.Duration
	NimiqNetwork    string
	MerchantAddress string
	NimiqRPCURL     string
}

// Enabled reports whether all public Nimiq verification settings are present.
func (p PaymentConfig) Enabled() bool {
	return strings.TrimSpace(p.NimiqNetwork) != "" &&
		strings.TrimSpace(p.MerchantAddress) != "" &&
		strings.TrimSpace(p.NimiqRPCURL) != ""
}

// Validate rejects partial or malformed payment configuration without accepting any private key.
func (p PaymentConfig) Validate() error {
	if strings.TrimSpace(p.NimiqNetwork) == "" &&
		strings.TrimSpace(p.MerchantAddress) == "" &&
		strings.TrimSpace(p.NimiqRPCURL) == "" {
		return nil
	}
	if !p.Enabled() {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK, NIMNEAR_MERCHANT_ADDRESS, and NIMNEAR_NIMIQ_RPC_URL must be set together")
	}
	parsed, err := url.Parse(p.NimiqRPCURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("NIMNEAR_NIMIQ_RPC_URL must be an absolute HTTP(S) URL")
	}
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.MerchantAddress), " ", ""))
	if len(normalized) != 36 || !strings.HasPrefix(normalized, "NQ") {
		return fmt.Errorf("NIMNEAR_MERCHANT_ADDRESS must be a 36-character Nimiq user-friendly address")
	}
	for _, char := range normalized[2:] {
		if !((char >= '0' && char <= '9') || (char >= 'A' && char <= 'Z')) {
			return fmt.Errorf("NIMNEAR_MERCHANT_ADDRESS contains unsupported characters")
		}
	}
	if strings.TrimSpace(p.NimiqNetwork) == "" || strings.Contains(p.NimiqNetwork, " ") {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK must be a non-empty network identifier")
	}
	return nil
}

// ValidateForEnvironment applies the existing payment validation plus the
// environment separation rules that prevent obvious testnet/development
// settings from being used in production. It does not redesign payment
// routing or infer a network from an RPC response.
func (p PaymentConfig) ValidateForEnvironment(environment string) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid payment configuration: %w", err)
	}
	if environment != EnvironmentProduction || !p.Enabled() {
		return nil
	}

	networkKind := classifyNimiqNetwork(p.NimiqNetwork)
	if networkKind != "main" {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK must identify the production main network")
	}

	parsed, err := url.Parse(p.NimiqRPCURL)
	if err != nil {
		return fmt.Errorf("NIMNEAR_NIMIQ_RPC_URL must be an absolute HTTP(S) URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "host.docker.internal" {
		return fmt.Errorf("NIMNEAR_NIMIQ_RPC_URL must not use a local development host in production")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return fmt.Errorf("NIMNEAR_NIMIQ_RPC_URL must not use a loopback host in production")
	}

	rpcKind := classifyNimiqRPCURL(p.NimiqRPCURL)
	if rpcKind == "test" {
		return fmt.Errorf("NIMNEAR_NIMIQ_RPC_URL must not identify a test network in production")
	}
	if rpcKind == "ambiguous" || (rpcKind != "unknown" && rpcKind != networkKind) {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK and NIMNEAR_NIMIQ_RPC_URL identify different networks")
	}
	return nil
}

func classifyNimiqNetwork(network string) string {
	normalized := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(network))
	switch normalized {
	case "mainnet", "mainalbatross":
		return "main"
	case "testnet", "testalbatross":
		return "test"
	default:
		return "unknown"
	}
}

func classifyNimiqRPCURL(rawURL string) string {
	value := strings.ToLower(rawURL)
	hasTest := strings.Contains(value, "testnet") || strings.Contains(value, "test-albatross") || strings.Contains(value, "testalbatross")
	hasMain := strings.Contains(value, "mainnet") || strings.Contains(value, "main-albatross") || strings.Contains(value, "mainalbatross")
	switch {
	case hasTest && hasMain:
		return "ambiguous"
	case hasTest:
		return "test"
	case hasMain:
		return "main"
	default:
		return "unknown"
	}
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// Load reads configuration from environment variables with development-safe
// defaults. Production must call Validate and cannot use these defaults.
func Load() *Config {
	return &Config{
		Environment: envOrDefault("APP_ENV", EnvironmentDevelopment),
		Server: ServerConfig{
			Host:               envOrDefault("SERVER_HOST", "0.0.0.0"),
			Port:               envOrDefaultInt("SERVER_PORT", 8080),
			ReadTimeout:        time.Duration(envOrDefaultInt("SERVER_READ_TIMEOUT_SECONDS", 15)) * time.Second,
			WriteTimeout:       time.Duration(envOrDefaultInt("SERVER_WRITE_TIMEOUT_SECONDS", 15)) * time.Second,
			IdleTimeout:        time.Duration(envOrDefaultInt("SERVER_IDLE_TIMEOUT_SECONDS", 60)) * time.Second,
			CORSAllowedOrigins: envOrDefaultSlice("CORS_ALLOWED_ORIGINS", nil),
			MaxBodyBytes:       envOrDefaultInt64("MAX_BODY_BYTES", 1<<20),
		},
		Database: DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefaultInt("DB_PORT", 5432),
			User:     envOrDefault("DB_USER", "masterfabric"),
			Password: envOrDefault("DB_PASSWORD", "masterfabric"),
			DBName:   envOrDefault("DB_NAME", "masterfabric"),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
			MaxConns: envOrDefaultInt32("DB_MAX_CONNS", 25),
			MinConns: envOrDefaultInt32("DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:     envOrDefault("REDIS_HOST", "localhost"),
			Port:     envOrDefaultInt("REDIS_PORT", 6379),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envOrDefaultInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:          envOrDefault("JWT_SECRET", defaultJWTSecret),
			ExpirationHours: envOrDefaultInt("JWT_EXPIRATION_HOURS", 24),
			Issuer:          envOrDefault("JWT_ISSUER", defaultJWTIssuer),
		},
		Kafka: KafkaConfig{
			Brokers:           envOrDefaultSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			GroupID:           envOrDefault("KAFKA_GROUP_ID", "masterfabric-go"),
			Enabled:           envOrDefault("KAFKA_ENABLED", "false") == "true",
			NumPartitions:     envOrDefaultInt("KAFKA_NUM_PARTITIONS", 3),
			ReplicationFactor: envOrDefaultInt("KAFKA_REPLICATION_FACTOR", 1),
		},
		WebSocket: WebSocketConfig{
			Enabled:         envOrDefault("WS_ENABLED", "true") == "true",
			MaxConnections:  envOrDefaultInt("WS_MAX_CONNECTIONS", 1000),
			PingIntervalSec: envOrDefaultInt("WS_PING_INTERVAL_SECONDS", 30),
			ReadBufferSize:  envOrDefaultInt("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize: envOrDefaultInt("WS_WRITE_BUFFER_SIZE", 1024),
		},
		Payments: PaymentConfig{
			HoldDuration:    time.Duration(envOrDefaultInt("NIMNEAR_PURCHASE_HOLD_MINUTES", 10)) * time.Minute,
			NimiqNetwork:    envOrDefault("NIMNEAR_NIMIQ_NETWORK", ""),
			MerchantAddress: envOrDefault("NIMNEAR_MERCHANT_ADDRESS", ""),
			NimiqRPCURL:     envOrDefault("NIMNEAR_NIMIQ_RPC_URL", ""),
		},
		Log: LogConfig{
			Level:  envOrDefault("LOG_LEVEL", "info"),
			Format: envOrDefault("LOG_FORMAT", "json"),
		},
	}
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func envOrDefaultInt32(key string, defaultVal int32) int32 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return defaultVal
}

func envOrDefaultInt64(key string, defaultVal int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return defaultVal
}

func envOrDefaultSlice(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		parts := strings.Split(val, ",")
		var result []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultVal
}
