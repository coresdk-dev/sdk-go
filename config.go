package coresdk

import "os"

// Config holds SDK configuration. All fields have safe defaults.
type Config struct {
	SidecarAddr string
	TenantID    string
	ServiceName string
	FailMode    string // "open" or "closed"
	DevMode     bool
	TLSCert     string
	TLSKey      string
	TLSCA       string
}

// ConfigFromEnv reads configuration from environment variables.
func ConfigFromEnv() *Config {
	devMode := os.Getenv("CORESDK_ENV") == "development"
	return &Config{
		SidecarAddr: envOrDefault("CORESDK_SIDECAR_ADDR", "localhost:50051"),
		TenantID:    envOrDefault("CORESDK_TENANT_ID", "default"),
		ServiceName: envOrDefault("CORESDK_SERVICE_NAME", "unknown-service"),
		FailMode:    envOrDefault("CORESDK_FAIL_MODE", "open"),
		DevMode:     devMode,
		TLSCert:     os.Getenv("CORESDK_TLS_CERT"),
		TLSKey:      os.Getenv("CORESDK_TLS_KEY"),
		TLSCA:       os.Getenv("CORESDK_TLS_CA"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
