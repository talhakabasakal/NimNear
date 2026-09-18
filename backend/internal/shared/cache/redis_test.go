package cache

import (
	"crypto/tls"
	"testing"

	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisOptions_TLSDisabledLeavesTLSConfigNil(t *testing.T) {
	opts := redisOptions(config.RedisConfig{
		Host:     "localhost",
		Port:     6379,
		Password: "secret",
		DB:       2,
		TLS:      false,
	})

	require.NotNil(t, opts)
	assert.Equal(t, "localhost:6379", opts.Addr)
	assert.Equal(t, "secret", opts.Password)
	assert.Equal(t, 2, opts.DB)
	assert.Nil(t, opts.TLSConfig)

	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	assert.Nil(t, client.Options().TLSConfig)
}

func TestRedisOptions_TLSEnabledUsesVerifiedTLS12(t *testing.T) {
	opts := redisOptions(config.RedisConfig{
		Host:     "redis.example.com",
		Port:     6379,
		Password: "secret",
		DB:       0,
		TLS:      true,
	})

	require.NotNil(t, opts)
	require.NotNil(t, opts.TLSConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), opts.TLSConfig.MinVersion)
	assert.False(t, opts.TLSConfig.InsecureSkipVerify)
	assert.Empty(t, opts.TLSConfig.ServerName)

	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	got := client.Options().TLSConfig
	require.NotNil(t, got)
	assert.Equal(t, uint16(tls.VersionTLS12), got.MinVersion)
	assert.False(t, got.InsecureSkipVerify)
}
