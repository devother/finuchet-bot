package database

import (
	"database/sql"
	"finuchet-bot/config"
	"fmt"

	_ "github.com/lib/pq"
)

func Connect(cfg config.DBConfig) (*sql.DB, error) {
	// Формируем строку подключения
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	// Открываем соединение с базой данных
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Проверяем соединение с базой данных
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	return db, nil
}
