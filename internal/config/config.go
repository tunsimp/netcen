package config

import "os"

type Config struct {
	HTTPPort  string
	DBPath    string
	JWTSecret string
}

func Load() Config {
	return Config{
		HTTPPort:  getOrDefault("HTTP_PORT", "8080"),
		DBPath:    getOrDefault("DB_PATH", "./data/mangahub.db"),
		JWTSecret: getOrDefault("JWT_SECRET", "dev-secret-change-me"),
	}
}

func getOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
