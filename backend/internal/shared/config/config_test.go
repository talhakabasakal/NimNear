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
	cfg := Load()

	assert.Equal(t, EnvironmentDevelopment, cfg.Environment)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "masterfabric", cfg.Database.User)
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, 30*time.Second, cfg.Payments.ReconciliationInterval)
	assert.Equal(t, time.Hour, cfg.Payments.ReconciliationDeadline)
	assert.Equal(t, 50, cfg.Payments.ReconciliationBatchSize)
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

func validProductionConfig() Config {
	return Config{
		Environment: EnvironmentProduction,
		Server:      ServerConfig{CORSAllowedOrigins: []string{"https://app.example.com"}},
		Database: DatabaseConfig{
			Host:     "postgres.internal",
			Port:     5432,
			User:     "nimnear_app",
			Password: "a-production-db-password",
			DBName:   "nimnear_production",
			SSLMode:  "require",
		},
		JWT: JWTConfig{
			Secret:          strings.Repeat("j", 32),
			ExpirationHours: 24,
			Issuer:          "nimnear-production",
		},
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
		MerchantAddress: "NQ07 0000 0000 0000 0000 0000 0000 0000 0000",
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
		MerchantAddress: "NQ07 0000 0000 0000 0000 0000 0000 0000 0000",
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
		MerchantAddress: "NQ07 0000 0000 0000 0000 0000 0000 0000 0000",
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
		MerchantAddress:         "NQ07 0000 0000 0000 0000 0000 0000 0000 0000",
		NimiqRPCURL:             "https://rpc.testnet.example",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected negative reconciliation interval to be rejected")
	}
}
