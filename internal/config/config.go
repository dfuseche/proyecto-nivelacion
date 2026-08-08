package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port           string
	JWTSecret      string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	RabbitMQURL    string
	RabbitMQQueue  string
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
}

func LoadConfig() *Config {
	useSSL, _ := strconv.ParseBool(getEnv("MINIO_USE_SSL", "false"))

	return &Config{
		Port:           getEnv("PORT", "8080"),
		JWTSecret:      getEnv("JWT_SECRET", "super-secret-okf-jwt-key-2026"),
		DBHost:         getEnv("DB_HOST", "postgres"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "okf_user"),
		DBPassword:     getEnv("DB_PASSWORD", "okf_password"),
		DBName:         getEnv("DB_NAME", "okf_db"),
		RabbitMQURL:    getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		RabbitMQQueue:  getEnv("RABBITMQ_QUEUE", "okf_jobs"),
		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:    getEnv("MINIO_BUCKET", "okf-bundles"),
		MinIOUseSSL:    useSSL,
	}
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

func getEnv(key, defaultValue string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultValue
}
