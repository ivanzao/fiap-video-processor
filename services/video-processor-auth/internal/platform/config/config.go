package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	TokenTTL    time.Duration
	BcryptCost  int
}

func Load() (Config, error) {
	var errs []error
	cfg := Config{
		Port:        env("PORT", "8081"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		TokenTTL:    envDuration("TOKEN_TTL", time.Hour, &errs),
		BcryptCost:  envInt("BCRYPT_COST", 10, &errs),
	}
	for name, value := range map[string]string{"DATABASE_URL": cfg.DatabaseURL, "JWT_SECRET": cfg.JWTSecret} {
		if value == "" {
			errs = append(errs, fmt.Errorf("config: %s is required", name))
		}
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func LoadVerifier() (Config, error) {
	cfg := Config{JWTSecret: os.Getenv("JWT_SECRET")}
	if cfg.JWTSecret == "" {
		return Config{}, errors.New("config: JWT_SECRET is required")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func envInt(name string, fallback int, errs *[]error) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: %s: %w", name, err))
	}
	return n
}

func envDuration(name string, fallback time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("config: %s: %w", name, err))
	}
	return d
}
