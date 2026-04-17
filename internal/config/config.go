package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL        string
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseJWTSecret  string
	StripeSecretKey    string
	StripeWebhookSecret string
	StripePriceID      string
	Port               string
	AdminUserID        string
}

func Load() (*Config, error) {
	c := &Config{
		DatabaseURL:         mustEnv("DATABASE_URL"),
		SupabaseURL:         mustEnv("SUPABASE_URL"),
		SupabaseAnonKey:     mustEnv("SUPABASE_ANON_KEY"),
		SupabaseJWTSecret:   mustEnv("SUPABASE_JWT_SECRET"),
		StripeSecretKey:     mustEnv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: mustEnv("STRIPE_WEBHOOK_SECRET"),
		StripePriceID:       mustEnv("STRIPE_PRICE_ID"),
		AdminUserID:         mustEnv("ADMIN_USER_ID"),
		Port:                getEnv("PORT", "8080"),
	}
	return c, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %s is not set", key))
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
