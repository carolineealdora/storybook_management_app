package config

import (
	"os"
	"testing"

	"github.com/caarlos0/env/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORSAllowOrigins_Unset_DefaultsToNoOrigins(t *testing.T) {
	if old, ok := os.LookupEnv("CORS_ALLOW_ORIGINS"); ok {
		require.NoError(t, os.Unsetenv("CORS_ALLOW_ORIGINS"))
		t.Cleanup(func() { _ = os.Setenv("CORS_ALLOW_ORIGINS", old) })
	}
	var cfg SystemConfig
	require.NoError(t, env.Parse(&cfg))
	assert.Empty(t, cfg.CORSAllowOrigins, "same-origin only unless origins are configured explicitly")
}

func TestCORSAllowOrigins_CommaSeparated(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "https://admin.example,https://staging-admin.example")
	var cfg SystemConfig
	require.NoError(t, env.Parse(&cfg))
	assert.Equal(t, []string{"https://admin.example", "https://staging-admin.example"}, cfg.CORSAllowOrigins)
}
