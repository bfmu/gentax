package config_test

import (
	"os"
	"testing"

	"github.com/bmunoz/gentax/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setEnv sets environment variables for the duration of a test and restores them on cleanup.
func setEnv(t *testing.T, pairs map[string]string) {
	t.Helper()
	original := make(map[string]string, len(pairs))
	for k := range pairs {
		original[k] = os.Getenv(k)
	}
	for k, v := range pairs {
		os.Setenv(k, v)
	}
	t.Cleanup(func() {
		for k, v := range original {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
}

// allRequiredVars returns a map of all required env vars with valid test values.
func allRequiredVars() map[string]string {
	return map[string]string{
		"DATABASE_URL":       "postgres://gentax:gentax@localhost:5432/gentax?sslmode=disable",
		"TELEGRAM_BOT_TOKEN": "test-bot-token",
		"JWT_SECRET":         "this-is-a-very-secure-secret-of-32plus-chars",
		"OCR_PROVIDER":       "google_vision",
		"STORAGE_BUCKET":     "test-bucket",
		"STORAGE_PROVIDER":   "gcs",
	}
}

// clearAllVars unsets all known env vars for the test.
func clearAllVars(t *testing.T) {
	t.Helper()
	keys := []string{
		"DATABASE_URL", "TELEGRAM_BOT_TOKEN", "JWT_SECRET",
		"OCR_PROVIDER", "GOOGLE_VISION_API_KEY", "OPENAI_API_KEY",
		"STORAGE_BUCKET", "STORAGE_PROVIDER",
		"APP_ENV", "API_PORT", "BOT_POLL_TIMEOUT",
		"OCR_WORKER_POOL_SIZE", "OCR_POLL_INTERVAL_SECS",
	}
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			}
		}
	})
}

func TestConfig_LoadsAllRequiredVars(t *testing.T) {
	clearAllVars(t)
	setEnv(t, allRequiredVars())

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "postgres://gentax:gentax@localhost:5432/gentax?sslmode=disable", cfg.DatabaseURL)
	assert.Equal(t, "test-bot-token", cfg.TelegramBotToken)
	assert.Equal(t, "this-is-a-very-secure-secret-of-32plus-chars", cfg.JWTSecret)
	assert.Equal(t, "google_vision", cfg.OCRProvider)
	assert.Equal(t, "test-bucket", cfg.StorageBucket)
	assert.Equal(t, "gcs", cfg.StorageProvider)
}

func TestConfig_ErrorOnMissingJWTSecret(t *testing.T) {
	clearAllVars(t)
	vars := allRequiredVars()
	delete(vars, "JWT_SECRET")
	setEnv(t, vars)

	cfg, err := config.Load()
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestConfig_ErrorOnMissingDatabaseURL(t *testing.T) {
	clearAllVars(t)
	vars := allRequiredVars()
	delete(vars, "DATABASE_URL")
	setEnv(t, vars)

	cfg, err := config.Load()
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL")
}

func TestConfig_DefaultValues(t *testing.T) {
	clearAllVars(t)
	setEnv(t, allRequiredVars())
	// Explicitly unset optionals so defaults kick in
	os.Unsetenv("APP_ENV")
	os.Unsetenv("API_PORT")
	os.Unsetenv("BOT_POLL_TIMEOUT")
	os.Unsetenv("OCR_WORKER_POOL_SIZE")
	os.Unsetenv("OCR_POLL_INTERVAL_SECS")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, 8080, cfg.APIPort)
	assert.Equal(t, 60, cfg.BotPollTimeout)
	assert.Equal(t, 3, cfg.OCRWorkerPoolSize)
	assert.Equal(t, 5, cfg.OCRPollIntervalSecs)
}

func TestConfig_ErrorOnMultipleMissingVars(t *testing.T) {
	clearAllVars(t)
	// Set only one required var; the rest should all be reported
	setEnv(t, map[string]string{
		"DATABASE_URL": "postgres://localhost/gentax",
	})

	cfg, err := config.Load()
	assert.Nil(t, cfg)
	require.Error(t, err)
	// Should mention multiple missing vars
	assert.Contains(t, err.Error(), "TELEGRAM_BOT_TOKEN")
	assert.Contains(t, err.Error(), "JWT_SECRET")
	assert.Contains(t, err.Error(), "OCR_PROVIDER")
}

func TestConfig_ErrorOnJWTSecretTooShort(t *testing.T) {
	clearAllVars(t)
	vars := allRequiredVars()
	vars["JWT_SECRET"] = "tooshort"
	setEnv(t, vars)

	cfg, err := config.Load()
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}
