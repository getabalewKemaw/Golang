package configs

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (dbConfig DBConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Name, dbConfig.SSLMode)
}

type Config struct {
	Port             string
	AppEnv           string
	DB               DBConfig
	JWTAccessSecret  string
	JWTRefreshSecret string
}

func Load() *Config {
	_ = godotenv.Load()
	dbConfig := DBConfig{
		Host:     getEnv("POSTGRES_HOST", "localhost"),
		Port:     getEnv("POSTGRES_PORT", "5432"),
		User:     getEnv("POSTGRES_USER", "postgres"),
		Password: getEnv("POSTGRES_PASSWORD", ""),
		Name:     getEnv("POSTGRES_DB", "golang"),
		SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
	}
	defaultSecret := getEnv("JWT_SECRET", "default_insecure_jwt_secret_change_me")

	return &Config{
		Port:             getEnv("PORT", "8080"),
		AppEnv:           getEnv("APP_ENV", "development"),
		DB:               dbConfig,
		JWTAccessSecret:  getEnv("JWT_ACCESS_SECRET", defaultSecret+"_access"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", defaultSecret+"_refresh"),
	}
}
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
