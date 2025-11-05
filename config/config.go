package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken string
	DB       DBConfig
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

func LoadConfig() *Config {
	err := godotenv.Load() // Переменные окружения из файла .env
	if err != nil {
		log.Printf("Не удалось загрузить файл .env, используем переменные окружения: %v", err)
	}

	return &Config{
		BotToken: getEnv("BOT_TOKEN", ""),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     5432,
			User:     getEnv("DB_USER", "postgre"),
			Password: getEnv("DB_PASSWORD", "root"),
			DBName:   getEnv("DB_NAME", "db_admin"),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("Переменная окружения %s не задана, используем значение по умолчанию.", key)
	return defaultVal
}
