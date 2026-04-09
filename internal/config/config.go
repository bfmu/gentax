package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Required
	DatabaseURL        string
	TelegramBotToken   string
	JWTSecret          string
	OCRProvider        string
	StorageBucket      string
	StorageProvider    string

	// Optional with defaults
	AppEnv              string
	APIPort             int
	BotPollTimeout      int
	OCRWorkerPoolSize   int
	OCRPollIntervalSecs int

	// Optional (provider-dependent)
	GoogleVisionAPIKey string
	OpenAIAPIKey       string
}

// Load reads all required environment variables and returns a populated Config.
// It collects ALL missing required vars before returning an error.
func Load() (*Config, error) {
	var missing []string

	getRequired := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		DatabaseURL:      getRequired("DATABASE_URL"),
		TelegramBotToken: getRequired("TELEGRAM_BOT_TOKEN"),
		JWTSecret:        getRequired("JWT_SECRET"),
		OCRProvider:      getRequired("OCR_PROVIDER"),
		StorageBucket:    getRequired("STORAGE_BUCKET"),
		StorageProvider:  getRequired("STORAGE_PROVIDER"),

		// Optional with defaults
		AppEnv:              getEnvWithDefault("APP_ENV", "development"),
		GoogleVisionAPIKey:  os.Getenv("GOOGLE_VISION_API_KEY"),
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
	}

	// Validate JWT_SECRET length (only if it was provided)
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	// Validate OCR_PROVIDER values
	if cfg.OCRProvider != "" {
		switch cfg.OCRProvider {
		case "google_vision", "openai", "tesseract":
			// valid
		default:
			return nil, fmt.Errorf("OCR_PROVIDER must be one of: google_vision, openai, tesseract; got %q", cfg.OCRProvider)
		}
	}

	// Validate STORAGE_PROVIDER values
	if cfg.StorageProvider != "" {
		switch cfg.StorageProvider {
		case "local", "gcs", "s3":
			// valid
		default:
			return nil, fmt.Errorf("STORAGE_PROVIDER must be one of: local, gcs, s3; got %q", cfg.StorageProvider)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	var parseErrors []string

	apiPort, err := parseIntWithDefault("API_PORT", 8080)
	if err != nil {
		parseErrors = append(parseErrors, err.Error())
	} else {
		cfg.APIPort = apiPort
	}

	botPollTimeout, err := parseIntWithDefault("BOT_POLL_TIMEOUT", 60)
	if err != nil {
		parseErrors = append(parseErrors, err.Error())
	} else {
		cfg.BotPollTimeout = botPollTimeout
	}

	ocrWorkerPoolSize, err := parseIntWithDefault("OCR_WORKER_POOL_SIZE", 3)
	if err != nil {
		parseErrors = append(parseErrors, err.Error())
	} else {
		cfg.OCRWorkerPoolSize = ocrWorkerPoolSize
	}

	ocrPollIntervalSecs, err := parseIntWithDefault("OCR_POLL_INTERVAL_SECS", 5)
	if err != nil {
		parseErrors = append(parseErrors, err.Error())
	} else {
		cfg.OCRPollIntervalSecs = ocrPollIntervalSecs
	}

	if len(parseErrors) > 0 {
		return nil, errors.New(strings.Join(parseErrors, "; "))
	}

	return cfg, nil
}

func getEnvWithDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func parseIntWithDefault(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid value for %s: %q must be an integer", key, v)
	}
	return n, nil
}
