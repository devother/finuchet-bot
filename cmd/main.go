package main

import (
	"finuchet-bot/config"
	"finuchet-bot/internal/handlers"
	"finuchet-bot/pkg/database"
	"log"
)

func main() {
	// Загружаем конфигурацию
	cfg := config.LoadConfig()

	// Инициализируем подключение к базе данных
	db, err := database.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}
	defer db.Close()

	// Инициализируем Telegram-бота
	bot, err := handlers.NewBotHandler(cfg.BotToken, db)
	if err != nil {
		log.Fatalf("Ошибка инициализации бота: %v", err)
	}

	// Запускаем обработку обновлений
	bot.Start()
}
