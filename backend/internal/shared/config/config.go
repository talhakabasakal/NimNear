package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/masterfabric-go/masterfabric/internal/shared/httpx"
	"github.com/masterfabric-go/masterfabric/internal/shared/nimiq"
)

const (
	EnvironmentDevelopment = "development"
	EnvironmentTest        = "test"
	EnvironmentProduction  = "production"

	defaultJWTSecret  = "change-me-in-production"
	defaultJWTIssuer  = "nimnear"
	defaultDBHost     = "localhost"
	defaultDBUser     = "nimnear"
	defaultDBPassword = "nimnear"
	defaultDBName     = "nimnear"
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
	NimiqAuth   NimiqAuthConfig
	EmailAuth   EmailAuthConfig
	Platform    PlatformConfig
	Metrics     MetricsConfig
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

	if err := c.NimiqAuth.Validate(environment); err != nil {
		return err
	}
	if err := c.Payments.ValidateForEnvironment(environment); err != nil {
		return err
	}
	if err := validateAuthPaymentNetworkConsistency(c.NimiqAuth, c.Payments); err != nil {
		return err
	}
	if _, err := c.Server.TrustedProxies(); err != nil {
		return err
	}
	if environment == EnvironmentProduction {
		if err := validateProductionRedis(c.Redis); err != nil {
			return err
		}
		if c.Metrics.Public {
			return fmt.Errorf("NIMNEAR_METRICS_PUBLIC must be false in production")
		}
		if c.EmailAuth.Enabled {
			return fmt.Errorf("NIMNEAR_EMAIL_AUTH_ENABLED must be false in production")
		}
		if c.Platform.APIEnabled {
			return fmt.Errorf("NIMNEAR_PLATFORM_API_ENABLED must be false in production")
		}
		if err := validateProductionPublicOrigin(c.Server); err != nil {
			return err
		}
	}
	return nil
}

func validateAuthPaymentNetworkConsistency(auth NimiqAuthConfig, payments PaymentConfig) error {
	if !payments.Enabled() {
		return nil
	}
	if !nimiq.SameEnvironment(auth.Network, payments.NimiqNetwork) {
		return fmt.Errorf("NIMNEAR_AUTH_NETWORK and NIMNEAR_NIMIQ_NETWORK must refer to the same Nimiq environment")
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

func validateProductionRedis(cfg RedisConfig) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("REDIS_HOST must be configured in production")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("REDIS_PORT must be a valid TCP port in production")
	}
	if !cfg.TLS {
		return fmt.Errorf("REDIS_TLS must be true in production")
	}
	return nil
}

func validateProductionPublicOrigin(cfg ServerConfig) error {
	origin := strings.TrimSpace(cfg.PublicOrigin)
	if origin == "" {
		return fmt.Errorf("NIMNEAR_PUBLIC_ORIGIN must be an explicit HTTPS frontend origin in production")
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("NIMNEAR_PUBLIC_ORIGIN must be an absolute HTTPS origin")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || strings.HasSuffix(host, ".vercel.app") {
		return fmt.Errorf("NIMNEAR_PUBLIC_ORIGIN must be the canonical production frontend origin")
	}
	if !containsExactOrigin(cfg.CORSAllowedOrigins, origin) {
		return fmt.Errorf("NIMNEAR_PUBLIC_ORIGIN must be included in CORS_ALLOWED_ORIGINS")
	}
	return nil
}

func containsExactOrigin(origins []string, want string) bool {
	normalized := strings.TrimRight(strings.TrimSpace(want), "/")
	for _, origin := range origins {
		if strings.TrimRight(strings.TrimSpace(origin), "/") == normalized {
			return true
		}
	}
	return false
}

func defaultWSAllowEmptyOrigin(environment string) string {
	if environment == EnvironmentProduction {
		return "false"
	}
	return "true"
}

// NimiqAuthConfig binds every wallet login challenge to one deployment.
type NimiqAuthConfig struct {
	Network          string
	Environment      string
	Domain           string
	ChallengeTTL     time.Duration
	CookieName       string
	CookieSecure     bool
	CookieSameSite   string
	ChallengeIPLimit int
	VerifyIPLimit    int
	AbuseLimit       int
	RateLimitWindow  time.Duration
}

func (c NimiqAuthConfig) Validate(environment string) error {
	parsed, err := nimiq.ParseNetwork(c.Network)
	if err != nil {
		return fmt.Errorf("NIMNEAR_AUTH_NETWORK must be test-albatross or main-albatross")
	}
	if strings.TrimSpace(c.Environment) == "" {
		return fmt.Errorf("NIMNEAR_AUTH_ENVIRONMENT must be configured")
	}
	if !strings.EqualFold(strings.TrimSpace(c.Environment), parsed.Environment) {
		return fmt.Errorf("NIMNEAR_AUTH_ENVIRONMENT must match NIMNEAR_AUTH_NETWORK")
	}
	if environment == EnvironmentProduction && !parsed.IsMain() {
		return fmt.Errorf("NIMNEAR_AUTH_NETWORK must identify the production main network")
	}
	if strings.TrimSpace(c.Domain) == "" {
		return fmt.Errorf("NIMNEAR_AUTH_DOMAIN must be configured")
	}
	if c.ChallengeTTL <= 0 || c.ChallengeTTL > 5*time.Minute {
		return fmt.Errorf("NIMNEAR_AUTH_CHALLENGE_TTL_SECONDS must be between 1 and 300")
	}
	switch strings.ToLower(strings.TrimSpace(c.CookieSameSite)) {
	case "lax", "strict", "none":
	default:
		return fmt.Errorf("NIMNEAR_AUTH_COOKIE_SAME_SITE must be lax, strict, or none")
	}
	if strings.EqualFold(c.CookieSameSite, "none") && !c.CookieSecure {
		return fmt.Errorf("NIMNEAR_AUTH_COOKIE_SECURE must be true when SameSite=None")
	}
	if environment == EnvironmentProduction && !c.CookieSecure {
		return fmt.Errorf("NIMNEAR_AUTH_COOKIE_SECURE must be true in production")
	}
	if c.ChallengeIPLimit < 0 || c.VerifyIPLimit < 0 || c.AbuseLimit < 0 {
		return fmt.Errorf("Nimiq authentication rate limits must not be negative")
	}
	if c.RateLimitWindow < 0 || c.RateLimitWindow > time.Hour {
		return fmt.Errorf("NIMNEAR_AUTH_RATE_LIMIT_WINDOW_SECONDS must be between 0 and 3600")
	}
	return nil
}

func (c NimiqAuthConfig) ChallengeLimit() int {
	if c.ChallengeIPLimit > 0 {
		return c.ChallengeIPLimit
	}
	return 10
}

func (c NimiqAuthConfig) VerifyLimit() int {
	if c.VerifyIPLimit > 0 {
		return c.VerifyIPLimit
	}
	return 10
}

func (c NimiqAuthConfig) VerifyAbuseLimit() int {
	if c.AbuseLimit > 0 {
		return c.AbuseLimit
	}
	return 5
}

func (c NimiqAuthConfig) AuthRateLimitWindow() time.Duration {
	if c.RateLimitWindow > 0 {
		return c.RateLimitWindow
	}
	return time.Minute
}

// EmailAuthConfig controls the legacy email/password IAM surface.
type EmailAuthConfig struct {
	Enabled bool
}

// PlatformConfig controls leftover MasterFabric organization/app/gateway APIs.
type PlatformConfig struct {
	APIEnabled            bool
	GatewayTableAllowlist []string
}

// MetricsConfig controls Prometheus scrape exposure on the public HTTP listener.
type MetricsConfig struct {
	Enabled bool
	Public  bool
}

func defaultAuthCookieSecure(environment string) string {
	if environment == EnvironmentProduction {
		return "true"
	}
	return "false"
}

func defaultAuthCookieSameSite(environment string) string {
	if environment == EnvironmentProduction {
		return "none"
	}
	return "lax"
}

// WebSocketConfig holds real-time WebSocket settings.
type WebSocketConfig struct {
	Enabled          bool
	MaxConnections   int
	PingIntervalSec  int
	ReadBufferSize   int
	WriteBufferSize  int
	AllowEmptyOrigin bool
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host               string
	Port               int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	CORSAllowedOrigins []string
	PublicOrigin       string
	MaxBodyBytes       int64
	TrustedProxyCIDRs  []string
}

// TrustedProxies parses the explicit reverse-proxy allowlist. An empty list
// means X-Forwarded-For and X-Real-IP are ignored. Private ranges are never
// implied; operators must list each trusted hop.
func (s ServerConfig) TrustedProxies() (httpx.TrustedProxies, error) {
	proxies, err := httpx.ParseTrustedProxies(s.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("NIMNEAR_TRUSTED_PROXY_CIDRS must contain valid IPv4/IPv6 addresses or CIDR prefixes")
	}
	return proxies, nil
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
	TLS      bool
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
	HoldDuration            time.Duration
	ReconciliationInterval  time.Duration
	ReconciliationDeadline  time.Duration
	ReconciliationBatchSize int
	RequestTTL              time.Duration
	RequestCreateLimit      int
	RequestLookupLimit      int
	RequestSubmitLimit      int
	RequestRateLimitWindow  time.Duration
	PurchaseCreateLimit     int
	PurchaseSubmitLimit     int
	PurchaseRateLimitWindow time.Duration
	NimiqNetwork            string
	MerchantAddress         string
	NimiqRPCURL             string
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
	if p.HoldDuration < 0 || p.ReconciliationInterval < 0 || p.ReconciliationDeadline < 0 || p.ReconciliationBatchSize < 0 ||
		p.RequestTTL < 0 || p.RequestCreateLimit < 0 || p.RequestLookupLimit < 0 || p.RequestSubmitLimit < 0 || p.RequestRateLimitWindow < 0 ||
		p.PurchaseCreateLimit < 0 || p.PurchaseSubmitLimit < 0 || p.PurchaseRateLimitWindow < 0 {
		return fmt.Errorf("payment timing and batch settings must not be negative")
	}
	if p.RequestTTL > 30*24*time.Hour {
		return fmt.Errorf("NIMNEAR_PAYMENT_REQUEST_TTL_HOURS must not exceed 720")
	}
	if p.RequestRateLimitWindow > time.Hour {
		return fmt.Errorf("NIMNEAR_PAYMENT_REQUEST_RATE_LIMIT_WINDOW_SECONDS must not exceed 3600")
	}
	if p.PurchaseRateLimitWindow > time.Hour {
		return fmt.Errorf("NIMNEAR_PURCHASE_RATE_LIMIT_WINDOW_SECONDS must not exceed 3600")
	}
	if p.ReconciliationInterval > 0 && p.ReconciliationInterval < 5*time.Second {
		return fmt.Errorf("NIMNEAR_PURCHASE_RECONCILIATION_INTERVAL_SECONDS must be at least 5 seconds")
	}
	if p.ReconciliationInterval > 0 && p.ReconciliationDeadline > 0 && p.ReconciliationDeadline <= p.ReconciliationInterval {
		return fmt.Errorf("NIMNEAR_PURCHASE_RECONCILIATION_DEADLINE_MINUTES must exceed the reconciliation interval")
	}
	if p.ReconciliationBatchSize > 1000 {
		return fmt.Errorf("NIMNEAR_PURCHASE_RECONCILIATION_BATCH_SIZE must not exceed 1000")
	}
	if !p.Enabled() {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK, NIMNEAR_MERCHANT_ADDRESS, and NIMNEAR_NIMIQ_RPC_URL must be set together")
	}
	parsed, err := url.Parse(p.NimiqRPCURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("NIMNEAR_NIMIQ_RPC_URL must be an absolute HTTP(S) URL")
	}
	if _, err := nimiq.NormalizeUserFriendlyAddress(p.MerchantAddress); err != nil {
		return fmt.Errorf("NIMNEAR_MERCHANT_ADDRESS must be a checksum-valid Nimiq user-friendly address")
	}
	if _, err := nimiq.ParseNetwork(p.NimiqNetwork); err != nil {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK must be TestAlbatross or MainAlbatross")
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

	if network, err := nimiq.ParseNetwork(p.NimiqNetwork); err != nil || !network.IsMain() {
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
	if rpcKind == "ambiguous" || (rpcKind != "unknown" && rpcKind != "main") {
		return fmt.Errorf("NIMNEAR_NIMIQ_NETWORK and NIMNEAR_NIMIQ_RPC_URL identify different networks")
	}
	return nil
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
	environment := envOrDefault("APP_ENV", EnvironmentDevelopment)
	emailAuthDefault := "true"
	platformAPIDefault := "true"
	metricsPublicDefault := "true"
	if environment == EnvironmentProduction {
		emailAuthDefault = "false"
		platformAPIDefault = "false"
		metricsPublicDefault = "false"
	}
	return &Config{
		Environment: environment,
		Server: ServerConfig{
			Host:               envOrDefault("SERVER_HOST", "0.0.0.0"),
			Port:               envOrDefaultInt("SERVER_PORT", 8080),
			ReadTimeout:        time.Duration(envOrDefaultInt("SERVER_READ_TIMEOUT_SECONDS", 15)) * time.Second,
			WriteTimeout:       time.Duration(envOrDefaultInt("SERVER_WRITE_TIMEOUT_SECONDS", 15)) * time.Second,
			IdleTimeout:        time.Duration(envOrDefaultInt("SERVER_IDLE_TIMEOUT_SECONDS", 60)) * time.Second,
			CORSAllowedOrigins: envOrDefaultSlice("CORS_ALLOWED_ORIGINS", nil),
			PublicOrigin:       envOrDefault("NIMNEAR_PUBLIC_ORIGIN", ""),
			MaxBodyBytes:       envOrDefaultInt64("MAX_BODY_BYTES", 1<<20),
			TrustedProxyCIDRs:  envOrDefaultSlice("NIMNEAR_TRUSTED_PROXY_CIDRS", nil),
		},
		Database: DatabaseConfig{
			Host:     envOrDefault("DB_HOST", "localhost"),
			Port:     envOrDefaultInt("DB_PORT", 5432),
			User:     envOrDefault("DB_USER", defaultDBUser),
			Password: envOrDefault("DB_PASSWORD", defaultDBPassword),
			DBName:   envOrDefault("DB_NAME", defaultDBName),
			SSLMode:  envOrDefault("DB_SSLMODE", "disable"),
			MaxConns: envOrDefaultInt32("DB_MAX_CONNS", 25),
			MinConns: envOrDefaultInt32("DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:     envOrDefault("REDIS_HOST", "localhost"),
			Port:     envOrDefaultInt("REDIS_PORT", 6379),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envOrDefaultInt("REDIS_DB", 0),
			TLS:      envOrDefault("REDIS_TLS", "false") == "true",
		},
		JWT: JWTConfig{
			Secret:          envOrDefault("JWT_SECRET", defaultJWTSecret),
			ExpirationHours: envOrDefaultInt("JWT_EXPIRATION_HOURS", 24),
			Issuer:          envOrDefault("JWT_ISSUER", defaultJWTIssuer),
		},
		Kafka: KafkaConfig{
			Brokers:           envOrDefaultSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			GroupID:           envOrDefault("KAFKA_GROUP_ID", "nimnear-go"),
			Enabled:           envOrDefault("KAFKA_ENABLED", "false") == "true",
			NumPartitions:     envOrDefaultInt("KAFKA_NUM_PARTITIONS", 3),
			ReplicationFactor: envOrDefaultInt("KAFKA_REPLICATION_FACTOR", 1),
		},
		WebSocket: WebSocketConfig{
			Enabled:          envOrDefault("WS_ENABLED", "true") == "true",
			MaxConnections:   envOrDefaultInt("WS_MAX_CONNECTIONS", 1000),
			PingIntervalSec:  envOrDefaultInt("WS_PING_INTERVAL_SECONDS", 30),
			ReadBufferSize:   envOrDefaultInt("WS_READ_BUFFER_SIZE", 1024),
			WriteBufferSize:  envOrDefaultInt("WS_WRITE_BUFFER_SIZE", 1024),
			AllowEmptyOrigin: envOrDefault("WS_ALLOW_EMPTY_ORIGIN", defaultWSAllowEmptyOrigin(environment)) == "true",
		},
		Payments: PaymentConfig{
			HoldDuration:            time.Duration(envOrDefaultInt("NIMNEAR_PURCHASE_HOLD_MINUTES", 10)) * time.Minute,
			ReconciliationInterval:  time.Duration(envOrDefaultInt("NIMNEAR_PURCHASE_RECONCILIATION_INTERVAL_SECONDS", 30)) * time.Second,
			ReconciliationDeadline:  time.Duration(envOrDefaultInt("NIMNEAR_PURCHASE_RECONCILIATION_DEADLINE_MINUTES", 60)) * time.Minute,
			ReconciliationBatchSize: envOrDefaultInt("NIMNEAR_PURCHASE_RECONCILIATION_BATCH_SIZE", 50),
			RequestTTL:              time.Duration(envOrDefaultInt("NIMNEAR_PAYMENT_REQUEST_TTL_HOURS", 24)) * time.Hour,
			RequestCreateLimit:      envOrDefaultInt("NIMNEAR_PAYMENT_REQUEST_CREATE_LIMIT", 20),
			RequestLookupLimit:      envOrDefaultInt("NIMNEAR_PAYMENT_REQUEST_LOOKUP_LIMIT", 60),
			RequestSubmitLimit:      envOrDefaultInt("NIMNEAR_PAYMENT_REQUEST_SUBMIT_LIMIT", 30),
			RequestRateLimitWindow:  time.Duration(envOrDefaultInt("NIMNEAR_PAYMENT_REQUEST_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
			PurchaseCreateLimit:     envOrDefaultInt("NIMNEAR_PURCHASE_CREATE_LIMIT", 20),
			PurchaseSubmitLimit:     envOrDefaultInt("NIMNEAR_PURCHASE_SUBMIT_LIMIT", 30),
			PurchaseRateLimitWindow: time.Duration(envOrDefaultInt("NIMNEAR_PURCHASE_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
			NimiqNetwork:            envOrDefault("NIMNEAR_NIMIQ_NETWORK", ""),
			MerchantAddress:         envOrDefault("NIMNEAR_MERCHANT_ADDRESS", ""),
			NimiqRPCURL:             envOrDefault("NIMNEAR_NIMIQ_RPC_URL", ""),
		},
		NimiqAuth: NimiqAuthConfig{
			Network:          envOrDefault("NIMNEAR_AUTH_NETWORK", "test-albatross"),
			Environment:      envOrDefault("NIMNEAR_AUTH_ENVIRONMENT", "testnet"),
			Domain:           envOrDefault("NIMNEAR_AUTH_DOMAIN", "nimnear.local"),
			ChallengeTTL:     time.Duration(envOrDefaultInt("NIMNEAR_AUTH_CHALLENGE_TTL_SECONDS", 300)) * time.Second,
			CookieName:       envOrDefault("NIMNEAR_AUTH_COOKIE_NAME", "nimnear_session"),
			CookieSecure:     envOrDefault("NIMNEAR_AUTH_COOKIE_SECURE", defaultAuthCookieSecure(environment)) == "true",
			CookieSameSite:   envOrDefault("NIMNEAR_AUTH_COOKIE_SAME_SITE", defaultAuthCookieSameSite(environment)),
			ChallengeIPLimit: envOrDefaultInt("NIMNEAR_AUTH_CHALLENGE_IP_LIMIT", 10),
			VerifyIPLimit:    envOrDefaultInt("NIMNEAR_AUTH_VERIFY_IP_LIMIT", 10),
			AbuseLimit:       envOrDefaultInt("NIMNEAR_AUTH_VERIFY_ABUSE_LIMIT", 5),
			RateLimitWindow:  time.Duration(envOrDefaultInt("NIMNEAR_AUTH_RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
		},
		EmailAuth: EmailAuthConfig{
			Enabled: envOrDefault("NIMNEAR_EMAIL_AUTH_ENABLED", emailAuthDefault) == "true",
		},
		Platform: PlatformConfig{
			APIEnabled:            envOrDefault("NIMNEAR_PLATFORM_API_ENABLED", platformAPIDefault) == "true",
			GatewayTableAllowlist: envOrDefaultSlice("NIMNEAR_GATEWAY_TABLE_ALLOWLIST", nil),
		},
		Metrics: MetricsConfig{
			Enabled: envOrDefault("NIMNEAR_METRICS_ENABLED", "true") == "true",
			Public:  envOrDefault("NIMNEAR_METRICS_PUBLIC", metricsPublicDefault) == "true",
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
