package main

import (
	"log/slog"
	"os"
)

type Config struct {
	Addr               string
	DBPath             string
	JWTSecret          string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	DemoLogin          bool
}

func LoadConfig() Config {
	cfg := Config{
		Addr:               envOr("REGISTRY_ADDR", ":8090"),
		DBPath:             envOr("REGISTRY_DB_PATH", "./registry.db"),
		JWTSecret:          os.Getenv("REGISTRY_JWT_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  envOr("GOOGLE_REDIRECT_URL", "http://localhost:8090/api/auth/google/callback"),
		FrontendURL:        envOr("FRONTEND_URL", "http://localhost:3000"),
		DemoLogin:          os.Getenv("REGISTRY_DEMO_LOGIN") == "true",
	}
	if cfg.JWTSecret == "" {
		if cfg.DemoLogin {
			cfg.JWTSecret = "dev-secret-change-me"
			slog.Warn("using default JWT secret; set REGISTRY_JWT_SECRET for production")
		} else {
			slog.Error("REGISTRY_JWT_SECRET is required when demo login is disabled")
			os.Exit(1)
		}
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
