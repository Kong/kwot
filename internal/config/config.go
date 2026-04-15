package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration values
type Config struct {
	// Kong connection settings
	KongAddr     string
	AdminUser    string
	AdminToken   string
	Base64UIDPwd string
	AuthMethod   string // RBAC or PASSWORD
	CAPath       string
	SSLVerify    bool
	Proxy        string

	// Directory settings
	ConfigDir string

	// Feature flags
	DeleteExistingUsers       bool
	DeleteExistingRoles       bool
	CreateRBACUsers           bool
	DeleteExistingRBACUsers   bool
	FeatureForceWipeWorkspace bool
	FeatureDeleteAllEnabled   bool

	// Logging
	LogLib string

	// Concurrency settings
	MaxConcurrentWorkspaces int

	// Retry settings
	MaxRetryAttempts int

	// HTTP timeout settings
	HTTPRequestTimeout int // seconds; used as the global HTTP client timeout
}

var globalConfig *Config

// LoadConfig loads configuration from environment file and environment variables
func LoadConfig(configFile string) error {
	// Load .env file if it exists
	if configFile == "" {
		configFile = ".env"
	}

	if _, err := os.Stat(configFile); err == nil {
		if err := godotenv.Load(configFile); err != nil {
			return fmt.Errorf("error loading config file: %w", err)
		}
	}

	// Parse configuration
	cfg := &Config{
		KongAddr:     getEnv("KONG_ADDR", "http://localhost:8001"),
		AdminUser:    getEnv("ADMIN_USER", ""),
		AdminToken:   getEnv("ADMIN_TOKEN", ""),
		Base64UIDPwd: getEnv("BASE64_UID_PWD", ""),
		AuthMethod:   getEnv("AUTH_METHOD", "RBAC"),
		CAPath:       getEnv("CA", ""),
		SSLVerify:    getEnvBool("SSL_VERIFY", true),
		Proxy:        getEnv("PROXY", ""),
		ConfigDir:    getEnv("CONFIG_DIR", "./config/"),
		LogLib:       getEnv("LOG_LIB", ""),

		DeleteExistingUsers:       getEnvBool("FEATURE_DELETE_EXISTING_USERS", false),
		DeleteExistingRoles:       getEnvBool("FEATURE_DELETE_EXISTING_ROLES", true),
		CreateRBACUsers:           getEnvBool("FEATURE_CREATE_RBAC_USERS", true),
		DeleteExistingRBACUsers:   getEnvBool("FEATURE_DELETE_EXISTING_RBAC_USERS", true),
		FeatureForceWipeWorkspace: getEnvBool("FEATURE_FORCE_WIPE_WORKSPACE", false),
		FeatureDeleteAllEnabled:   getEnvBool("FEATURE_DELETE_ALL_ENABLED", false),

		MaxConcurrentWorkspaces: getEnvInt("MAX_CONCURRENT_WORKSPACES", 5),
		MaxRetryAttempts:        getEnvInt("MAX_RETRY_ATTEMPTS", 5),
		HTTPRequestTimeout:      getEnvInt("KONG_REQUEST_TIMEOUT", 30),
	}

	// Ensure MaxRetryAttempts is positive; fall back to default if misconfigured
	if cfg.MaxRetryAttempts <= 0 {
		cfg.MaxRetryAttempts = 5
	}

	// Ensure HTTPRequestTimeout is positive; 0 disables Go's HTTP client timeout entirely
	if cfg.HTTPRequestTimeout <= 0 {
		cfg.HTTPRequestTimeout = 30
	}

	// Validate required fields
	if cfg.AuthMethod == "" {
		return fmt.Errorf("ERROR: AUTH_METHOD not set\nMust be one of: RBAC or PASSWORD\nExample: export AUTH_METHOD=RBAC\nThen set corresponding credentials:\n  - For RBAC: export ADMIN_TOKEN=<token>\n  - For PASSWORD: export ADMIN_USER=<user> BASE64_UID_PWD=<base64_credentials>")
	}

	if cfg.AuthMethod == "PASSWORD" {
		if cfg.AdminUser == "" {
			return fmt.Errorf("ERROR: ADMIN_USER not set\nRequired when AUTH_METHOD=PASSWORD\nExample: export ADMIN_USER=kong_admin")
		}
		if cfg.Base64UIDPwd == "" {
			return fmt.Errorf("ERROR: BASE64_UID_PWD not set\nRequired when AUTH_METHOD=PASSWORD\nValue should be base64(username:password)\nExample: echo -n 'kong_admin:password' | base64")
		}
	}

	if cfg.AuthMethod == "RBAC" && cfg.AdminToken == "" {
		return fmt.Errorf("ERROR: ADMIN_TOKEN not set\nRequired when AUTH_METHOD=RBAC\nExample: export ADMIN_TOKEN=<your-kong-rbac-token>\nVerify token has 'admin' role in Kong Manager")
	}

	globalConfig = cfg
	return nil
}

// GetConfig returns the global configuration
func GetConfig() *Config {
	return globalConfig
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return defaultValue
		}
		return parsed
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return defaultValue
		}
		return parsed
	}
	return defaultValue
}
