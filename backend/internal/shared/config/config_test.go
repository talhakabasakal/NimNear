package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentDevelopment)
	t.Setenv("NIMNEAR_EMAIL_AUTH_ENABLED", "")
	t.Setenv("NIMNEAR_PLATFORM_API_ENABLED", "")
	t.Setenv("NIMNEAR_METRICS_PUBLIC", "")
	t.Setenv("REDIS_TLS", "")
	cfg := Load()

	assert.Equal(t, EnvironmentDevelopment, cfg.Environment)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "nimnear", cfg.Database.User)
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.False(t, cfg.Redis.TLS)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, 30*time.Second, cfg.Payments.ReconciliationInterval)
	assert.Equal(t, time.Hour, cfg.Payments.ReconciliationDeadline)
	assert.Equal(t, 50, cfg.Payments.ReconciliationBatchSize)
	assert.Equal(t, 24*time.Hour, cfg.Payments.RequestTTL)
	assert.Equal(t, 20, cfg.Payments.RequestCreateLimit)
	assert.Equal(t, 60, cfg.Payments.RequestLookupLimit)
	assert.Equal(t, 30, cfg.Payments.RequestSubmitLimit)
	assert.Equal(t, time.Minute, cfg.Payments.RequestRateLimitWindow)
	assert.Equal(t, 20, cfg.Payments.PurchaseCreateLimit)
	assert.Equal(t, 30, cfg.Payments.PurchaseSubmitLimit)
	assert.Equal(t, time.Minute, cfg.Payments.PurchaseRateLimitWindow)
	assert.Empty(t, cfg.Server.TrustedProxyCIDRs)
	assert.Equal(t, 10, cfg.NimiqAuth.ChallengeIPLimit)
	assert.Equal(t, 10, cfg.NimiqAuth.VerifyIPLimit)
	assert.Equal(t, 5, cfg.NimiqAuth.AbuseLimit)
	assert.Equal(t, time.Minute, cfg.NimiqAuth.RateLimitWindow)
	assert.Equal(t, "lax", cfg.NimiqAuth.CookieSameSite)
	assert.False(t, cfg.NimiqAuth.CookieSecure)
	assert.True(t, cfg.EmailAuth.Enabled)
	assert.True(t, cfg.Platform.APIEnabled)
	assert.True(t, cfg.Metrics.Enabled)
	assert.True(t, cfg.Metrics.Public)
	assert.Empty(t, cfg.Platform.GatewayTableAllowlist)
}

func TestLoad_ProductionDisablesLegacySurfacesByDefault(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentProduction)
	t.Setenv("NIMNEAR_EMAIL_AUTH_ENABLED", "")
	t.Setenv("NIMNEAR_PLATFORM_API_ENABLED", "")
	t.Setenv("NIMNEAR_METRICS_PUBLIC", "")
	cfg := Load()
	assert.False(t, cfg.EmailAuth.Enabled)
	assert.False(t, cfg.Platform.APIEnabled)
	assert.True(t, cfg.Metrics.Enabled)
	assert.False(t, cfg.Metrics.Public)
}

func TestLoad_ExplicitLegacySurfaceOverrides(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentProduction)
	t.Setenv("NIMNEAR_EMAIL_AUTH_ENABLED", "true")
	t.Setenv("NIMNEAR_PLATFORM_API_ENABLED", "true")
	t.Setenv("NIMNEAR_GATEWAY_TABLE_ALLOWLIST", "orders, catalog_items")
	cfg := Load()
	assert.True(t, cfg.EmailAuth.Enabled)
	assert.True(t, cfg.Platform.APIEnabled)
	assert.Equal(t, []string{"orders", "catalog_items"}, cfg.Platform.GatewayTableAllowlist)
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DB_HOST", "db.example.com")
	defer os.Unsetenv("SERVER_PORT")
	defer os.Unsetenv("DB_HOST")

	cfg := Load()
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
}

func TestLoad_DBPoolInt32Bounds(t *testing.T) {
	os.Setenv("DB_MAX_CONNS", "50")
	os.Setenv("DB_MIN_CONNS", "2147483648")
	defer os.Unsetenv("DB_MAX_CONNS")
	defer os.Unsetenv("DB_MIN_CONNS")

	cfg := Load()
	assert.Equal(t, int32(50), cfg.Database.MaxConns)
	assert.Equal(t, int32(5), cfg.Database.MinConns)
}

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "pass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}
	expected := "postgres://user:pass@localhost:5432/testdb?sslmode=disable"
	assert.Equal(t, expected, cfg.DSN())
}

func TestDatabaseConfig_DSN_EscapesSpecialCharacters(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user@domain",
		Password: "p@ss:w?rd#",
		DBName:   "testdb",
		SSLMode:  "require",
	}
	dsn := cfg.DSN()
	assert.Contains(t, dsn, "postgres://")
	assert.Contains(t, dsn, "sslmode=require")
	assert.NotContains(t, dsn, "p@ss:w?rd#")
}

func TestRedisConfig_Addr(t *testing.T) {
	cfg := RedisConfig{Host: "redis.local", Port: 6380}
	assert.Equal(t, "redis.local:6380", cfg.Addr())
}

func TestLoad_RedisTLSTrue(t *testing.T) {
	t.Setenv("REDIS_TLS", "true")
	cfg := Load()
	assert.True(t, cfg.Redis.TLS)
}

func TestLoad_RedisTLSDefaultsFalse(t *testing.T) {
	t.Setenv("REDIS_TLS", "")
	cfg := Load()
	assert.False(t, cfg.Redis.TLS)
}

func TestLoad_RedisTLSExplicitFalse(t *testing.T) {
	t.Setenv("REDIS_TLS", "false")
	cfg := Load()
	assert.False(t, cfg.Redis.TLS)
}

func validProductionConfig() Config {
	return Config{
		Environment: EnvironmentProduction,
		Server: ServerConfig{
			CORSAllowedOrigins: []string{"https://app.example.com"},
			PublicOrigin:       "https://app.example.com",
		},
		Database: DatabaseConfig{
			Host:     "postgres.internal",
			Port:     5432,
			User:     "nimnear_app",
			Password: "a-production-db-password",
			DBName:   "nimnear_production",
			SSLMode:  "require",
		},
		Redis: RedisConfig{Host: "redis.internal", Port: 6379, TLS: true},
		JWT: JWTConfig{
			Secret:          strings.Repeat("j", 32),
			ExpirationHours: 24,
			Issuer:          "nimnear-production",
		},
		NimiqAuth: NimiqAuthConfig{Network: "main-albatross", Environment: "mainnet", Domain: "app.example.com", ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSecure: true, CookieSameSite: "lax"},
	}
}

func TestConfigValidate_DevelopmentRemainsUsable(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentDevelopment)
	cfg := Load()

	assert.NoError(t, cfg.Validate())
}

func TestConfigValidate_ProductionMissingJWTSecretRejected(t *testing.T) {
	cfg := validProductionConfig()
	cfg.JWT.Secret = ""

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestConfigValidate_ProductionDefaultJWTSecretRejected(t *testing.T) {
	cfg := validProductionConfig()
	cfg.JWT.Secret = defaultJWTSecret

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
	assert.NotContains(t, err.Error(), defaultJWTSecret)
}

func TestConfigValidate_ProductionDefaultDatabaseRejected(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Database = DatabaseConfig{
		Host:     defaultDBHost,
		Port:     5432,
		User:     defaultDBUser,
		Password: defaultDBPassword,
		DBName:   defaultDBName,
		SSLMode:  defaultDBSSLMode,
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DB_HOST")
	assert.NotContains(t, err.Error(), defaultDBPassword)
}

func TestConfigValidate_ProductionExplicitConfigurationAccepted(t *testing.T) {
	cfg := validProductionConfig()

	assert.NoError(t, cfg.Validate())
}

func TestConfigValidate_DoesNotExposeSecrets(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Database.Host = defaultDBHost
	cfg.Database.Password = "do-not-log-this-password"

	err := cfg.Validate()
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), cfg.Database.Password)
	assert.NotContains(t, err.Error(), cfg.JWT.Secret)
}

func TestPaymentConfigValidateForEnvironment_RejectsTestNetworkInProduction(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Payments = PaymentConfig{
		NimiqNetwork:    "TestAlbatross",
		MerchantAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		NimiqRPCURL:     "https://rpc.testnet.example",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_NIMIQ_NETWORK")
}

func TestPaymentConfigValidateForEnvironment_RejectsNetworkMismatch(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Payments = PaymentConfig{
		NimiqNetwork:    "MainAlbatross",
		MerchantAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		NimiqRPCURL:     "https://rpc.testnet.example",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_NIMIQ_RPC_URL")
}

func TestPaymentConfigValidateForEnvironment_RejectsLocalRPCInProduction(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Payments = PaymentConfig{
		NimiqNetwork:    "MainAlbatross",
		MerchantAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		NimiqRPCURL:     "http://localhost:8648",
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_NIMIQ_RPC_URL")
}

func TestPaymentConfigValidate_RejectsNegativeReconciliationSettings(t *testing.T) {
	cfg := PaymentConfig{
		HoldDuration:            time.Minute,
		ReconciliationInterval:  -time.Second,
		ReconciliationDeadline:  time.Hour,
		ReconciliationBatchSize: 10,
		NimiqNetwork:            "TestAlbatross",
		MerchantAddress:         "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
		NimiqRPCURL:             "https://rpc.testnet.example",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative reconciliation interval to be rejected")
	}
}

func TestConfigValidate_NimiqNetworkMatrix(t *testing.T) {
	enabledPayments := func(network, rpcURL string) PaymentConfig {
		return PaymentConfig{
			NimiqNetwork:    network,
			MerchantAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
			NimiqRPCURL:     rpcURL,
		}
	}
	development := func(authNetwork, authEnv string, payments PaymentConfig) Config {
		return Config{
			Environment: EnvironmentDevelopment,
			NimiqAuth:   NimiqAuthConfig{Network: authNetwork, Environment: authEnv, Domain: "nimnear.local", ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSameSite: "lax"},
			Payments:    payments,
		}
	}

	t.Run("development auth test + payment test", func(t *testing.T) {
		cfg := development("test-albatross", "testnet", enabledPayments("TestAlbatross", "https://rpc.testnet.nimiqwatch.com"))
		assert.NoError(t, cfg.Validate())
	})
	t.Run("development auth main + payment main", func(t *testing.T) {
		cfg := development("main-albatross", "mainnet", enabledPayments("MainAlbatross", "https://rpc.example.com"))
		assert.NoError(t, cfg.Validate())
	})
	t.Run("auth test + payment main", func(t *testing.T) {
		cfg := development("test-albatross", "testnet", enabledPayments("MainAlbatross", "https://rpc.example.com"))
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NIMNEAR_AUTH_NETWORK")
		assert.Contains(t, err.Error(), "NIMNEAR_NIMIQ_NETWORK")
		assert.NotContains(t, err.Error(), "rpc.example.com")
	})
	t.Run("auth main + payment test", func(t *testing.T) {
		cfg := development("main-albatross", "mainnet", enabledPayments("TestAlbatross", "https://rpc.testnet.nimiqwatch.com"))
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NIMNEAR_AUTH_NETWORK")
		assert.NotContains(t, err.Error(), "rpc.testnet.nimiqwatch.com")
	})
	t.Run("production auth main + payment main", func(t *testing.T) {
		cfg := validProductionConfig()
		cfg.Payments = enabledPayments("MainAlbatross", "https://rpc.example.com")
		assert.NoError(t, cfg.Validate())
	})
	t.Run("production auth test + payment test", func(t *testing.T) {
		cfg := validProductionConfig()
		cfg.NimiqAuth.Network = "test-albatross"
		cfg.NimiqAuth.Environment = "testnet"
		cfg.Payments = enabledPayments("TestAlbatross", "https://rpc.testnet.nimiqwatch.com")
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NIMNEAR_AUTH_NETWORK")
		assert.NotContains(t, err.Error(), "rpc.testnet.nimiqwatch.com")
	})
	t.Run("production auth test + payment main", func(t *testing.T) {
		cfg := validProductionConfig()
		cfg.NimiqAuth.Network = "test-albatross"
		cfg.NimiqAuth.Environment = "testnet"
		cfg.Payments = enabledPayments("MainAlbatross", "https://rpc.example.com")
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NIMNEAR_AUTH_NETWORK")
	})
	t.Run("unknown auth network", func(t *testing.T) {
		cfg := development("ethereum", "testnet", PaymentConfig{})
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NIMNEAR_AUTH_NETWORK")
	})
	t.Run("unknown payment network", func(t *testing.T) {
		cfg := development("test-albatross", "testnet", enabledPayments("DevNet", "https://rpc.example.com"))
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "NIMNEAR_NIMIQ_NETWORK")
		assert.NotContains(t, err.Error(), "rpc.example.com")
	})
	t.Run("mismatch does not expose RPC credentials", func(t *testing.T) {
		cfg := development("test-albatross", "testnet", enabledPayments("MainAlbatross", "https://user:super-secret-rpc-token@rpc.example.com"))
		err := cfg.Validate()
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "super-secret-rpc-token")
		assert.NotContains(t, err.Error(), "user:super-secret")
	})
}

func TestLoad_ProductionCookieDefaults(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentProduction)
	t.Setenv("JWT_SECRET", strings.Repeat("j", 32))
	t.Setenv("JWT_ISSUER", "nimnear-production")
	t.Setenv("NIMNEAR_AUTH_COOKIE_SECURE", "")
	t.Setenv("NIMNEAR_AUTH_COOKIE_SAME_SITE", "")
	cfg := Load()
	assert.True(t, cfg.NimiqAuth.CookieSecure)
	assert.Equal(t, "none", cfg.NimiqAuth.CookieSameSite)
}

func TestTrustedProxyCIDRsRejectInvalidValues(t *testing.T) {
	cfg := Config{
		Environment: EnvironmentDevelopment,
		NimiqAuth:   NimiqAuthConfig{Network: "test-albatross", Environment: "testnet", Domain: "nimnear.local", ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSameSite: "lax"},
		Server:      ServerConfig{TrustedProxyCIDRs: []string{"10.0.0.0/8", "not-a-cidr"}},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_TRUSTED_PROXY_CIDRS")
	assert.NotContains(t, err.Error(), "not-a-cidr")
}

func TestTrustedProxyCIDRsAcceptIPv4AndIPv6(t *testing.T) {
	cfg := Config{
		Environment: EnvironmentDevelopment,
		NimiqAuth:   NimiqAuthConfig{Network: "test-albatross", Environment: "testnet", Domain: "nimnear.local", ChallengeTTL: 5 * time.Minute, CookieName: "nimnear_session", CookieSameSite: "lax"},
		Server:      ServerConfig{TrustedProxyCIDRs: []string{"10.0.0.1/32", "127.0.0.1", "::1/128", "2001:db8::/32"}},
	}
	assert.NoError(t, cfg.Validate())
	proxies, err := cfg.Server.TrustedProxies()
	assert.NoError(t, err)
	assert.Len(t, proxies, 4)
}

func TestConfigValidate_ProductionRejectsPublicMetricsEmailAndPlatform(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Metrics.Public = true
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_METRICS_PUBLIC")

	cfg = validProductionConfig()
	cfg.EmailAuth.Enabled = true
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_EMAIL_AUTH_ENABLED")

	cfg = validProductionConfig()
	cfg.Platform.APIEnabled = true
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_PLATFORM_API_ENABLED")
}

func TestConfigValidate_ProductionRejectsRedisWithoutTLS(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Redis.TLS = false

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS_TLS")
}

func TestConfigValidate_ProductionAcceptsRedisTLS(t *testing.T) {
	cfg := validProductionConfig()
	assert.True(t, cfg.Redis.TLS)
	assert.NoError(t, cfg.Validate())
}

func TestConfigValidate_DevelopmentAllowsRedisWithoutTLS(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentDevelopment)
	t.Setenv("REDIS_TLS", "false")
	cfg := Load()
	assert.False(t, cfg.Redis.TLS)
	assert.NoError(t, cfg.Validate())
}

func TestConfigValidate_ProductionRequiresRedisAndCanonicalOrigin(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Redis.Host = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "REDIS_HOST")

	cfg = validProductionConfig()
	cfg.Server.PublicOrigin = "http://app.example.com"
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_PUBLIC_ORIGIN")

	cfg = validProductionConfig()
	cfg.Server.PublicOrigin = "https://nimnear.vercel.app"
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_PUBLIC_ORIGIN")

	cfg = validProductionConfig()
	cfg.Server.PublicOrigin = "https://other.example.com"
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CORS_ALLOWED_ORIGINS")
}

func TestPaymentConfigValidate_RejectsInvalidMerchantChecksum(t *testing.T) {
	cfg := PaymentConfig{
		NimiqNetwork:    "TestAlbatross",
		MerchantAddress: "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T605",
		NimiqRPCURL:     "https://rpc.testnet.example",
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NIMNEAR_MERCHANT_ADDRESS")
	assert.NotContains(t, err.Error(), "NQ07")
}

func TestLoad_ProductionWebSocketOriginDefault(t *testing.T) {
	t.Setenv("APP_ENV", EnvironmentProduction)
	t.Setenv("WS_ALLOW_EMPTY_ORIGIN", "")
	cfg := Load()
	assert.False(t, cfg.WebSocket.AllowEmptyOrigin)
}
