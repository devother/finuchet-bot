package handlers

import (
	"database/sql"
	"finuchet-bot/internal/repository"
	"finuchet-bot/internal/services"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotHandler struct {
	bot         *tgbotapi.BotAPI
	service     *services.FinanceService
	userStates  map[int64]string  // Состояние пользователя
	userAmounts map[int64]float64 // Временное хранение суммы для пользователя
}

const (
	StateNone            = ""
	StateWaitingIncome   = "waiting_income"   // Состояние ожидания суммы для дохода
	StateWaitingExpense  = "waiting_expense"  // Состояние ожидания суммы для расхода
	StateIncomeCategory  = "income_category"  // Состояние ожидания категории дохода
	StateExpenseCategory = "expense_category" // Состояние ожидания категории расхода
)

func NewBotHandler(token string, db *sql.DB) (*BotHandler, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	repo := repository.NewPostgresRepository(db)
	service := services.NewFinanceService(repo)

	return &BotHandler{
		bot:         bot,
		service:     service,
		userStates:  make(map[int64]string),
		userAmounts: make(map[int64]float64),
	}, nil
}

func (h *BotHandler) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := h.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			h.handleCallbackQuery(update.CallbackQuery)
		} else if update.Message != nil {
			h.handleTransactionInput(update.Message)
		}
	}
}

// Обработка ввода данных в зависимости от состояния
func (h *BotHandler) handleTransactionInput(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text

	// Если сообщение из группового чата:
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		// if !strings.Contains(text, "@"+h.bot.Self.UserName) {
		// 	log.Printf("Игнорируем сообщения без упоминания бота")
		// 	return // Игнорируем сообщения без упоминания бота
		// }
		text = strings.ReplaceAll(text, "@"+h.bot.Self.UserName, "")
		text = strings.TrimSpace(text) // Убираем лишние пробелы после удаления упоминания
	}

	switch text {
	case "/start":
		if err := h.service.RegisterUser(chatID); err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при регистрации, попробуйте позже."))
			log.Printf("Ошибка регистрации пользователя: %v", err)
		} else {
			h.sendMainMenu(chatID)
		}
	case "/menu":
		h.sendMainMenu(chatID)
	case "/options":
		h.sendOptionMenu(chatID)
	case "/cancel":
		h.resetState(chatID) // Сброс состояния пользователя
		h.bot.Send(tgbotapi.NewMessage(chatID, "Действие отменено. Вы возвращены в главное меню."))
		h.sendMainMenu(chatID) // Отправляем главное меню
		return
	}

	currentState := h.userStates[chatID]

	switch currentState {
	case StateWaitingIncome, StateWaitingExpense:
		amount, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil || amount <= 0 {
			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
			return
		}

		h.userAmounts[chatID] = amount
		if currentState == StateWaitingIncome {
			h.userStates[chatID] = StateIncomeCategory
			h.sendIncomeCategories(chatID)
		} else {
			h.userStates[chatID] = StateExpenseCategory
			h.sendExpenseCategories(chatID)
		}

		// default:
		// 	h.sendMainMenu(chatID)
	}
}

// Отправка главного меню с кнопками "Доход", "Расход" и "Отчет"
func (h *BotHandler) sendMainMenu(chatID int64) {
	buttons := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Доход 📈", "income"),
			tgbotapi.NewInlineKeyboardButtonData("Расход 📉", "expense"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Отчет 📊", "report"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = buttons
	h.bot.Send(msg)
}

// Обработка CallbackQuery
func (h *BotHandler) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	chatID := callbackQuery.Message.Chat.ID
	data := callbackQuery.Data
	// text := callbackQuery.Message.Text

	switch data {
	case "income":
		h.userStates[chatID] = StateWaitingIncome
		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму дохода:"))

	case "expense":
		h.userStates[chatID] = StateWaitingExpense
		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму расхода:"))

	case "report":
		h.handleReportCommand(chatID)

	case "clear":
		h.handleClearData(chatID)

	case "salary", "debit", "invest", "deposit":
		h.addIncome(chatID, data)

	case "shop", "service", "cafe", "link", "educ":
		h.addExpense(chatID, data)
	}

	// Отметим callback как обработанный
	callbackConfig := tgbotapi.NewCallback(callbackQuery.ID, "")
	if _, err := h.bot.Request(callbackConfig); err != nil {
		log.Printf("Ошибка при отправке CallbackQuery ответа: %v", err)
	}
}

// Отправка меню для /utils
func (h *BotHandler) sendOptionMenu(chatID int64) {
	buttons := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Редактирование 📝", "edit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Выгрузка 📤", "export"),
			tgbotapi.NewInlineKeyboardButtonData("Очистка 🧹", "clear"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = buttons
	h.bot.Send(msg)
}

// Функция для очистки данных
func (h *BotHandler) handleClearData(chatID int64) {
	if err := h.service.ClearData(chatID); err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при очистке данных."))
		log.Printf("Ошибка при очистке данных: %v", err)
	} else {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Данные успешно очищены."))
	}
}

// Функция для выгрузки данных
// func (h *BotHandler) handleExportData(chatID int64) {
// 	data, err := h.service.ExportData(chatID)
// 	if err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при выгрузке данных."))
// 		log.Printf("Ошибка при выгрузке данных: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ваши данные:\n"+data))
// 	}
// }

// Отправка кнопок категорий для доходов
func (h *BotHandler) sendIncomeCategories(chatID int64) {
	buttons := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("З/п 💸", "salary"),
			tgbotapi.NewInlineKeyboardButtonData("Дебитор 🫴", "debit"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Премия 💰", "prize"),
			tgbotapi.NewInlineKeyboardButtonData("Подработка 🤑", "addinc"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Инвест 💹", "invest"),
			tgbotapi.NewInlineKeyboardButtonData("Вклад 🏦", "deposit"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Выберите категорию дохода:")
	msg.ReplyMarkup = buttons
	h.bot.Send(msg)
}

// Отправка кнопок категорий для расходов
func (h *BotHandler) sendExpenseCategories(chatID int64) {
	categories := [][]string{
		{"Аптеки 🏥", "phar"}, {"Авиабилеты 🛫", "avia"},
		{"Аксессуары 🕶️", "access"}, {"Анализы 💉", "analys"},
		{"Аренда 🔑", "rent"}, {"БытХим 🧹", "household"},
		{"Витамины 💊", "vitamin"}, {"Госуслуги 🏢", "state"},
		{"Дом и ремонт 🛠️", "repair"}, {"Ж/д билеты 🚂", "rail"},
		{"Животные 🐾", "animal"}, {"ЖКХ 👾", "service"},
		{"Инвестиции 💹", "invest"}, {"Интернет 🌐", "network"},
		{"Канцтовары 📝", "office"}, {"Каршеринг 🏎️", "carsh"},
		{"Книги 📚", "book"}, {"Красота 😻", "beauty"},
		{"Кредиты 💸", "Loan"}, {"Медицина 🩺", "medic"},
		{"Моб. связь 📞", "mobile"}, {"Наличные 🗞️", "cash"},
		{"Образование 🎓", "educ"}, {"Одежда и обувь👟", "clothes"},
		{"Переводы 📤", "trans"}, {"Подарки 🎁", "gift"},
		{"Подписки 🤳", "subscript"}, {"Развлечения 🎢", "fun"},
		{"Еда 🍜", "eat"}, {"Супермаркет 🛒", "mall"},
		{"Такси 🚕", "taxi"}, {"Топливо ⛽️", "oil"},
		{"Транспорт 🚌", "transport"}, {"Цветы 💐", "flowers"},
		{"Спорт 💪", "sport"}, {"Остальное 🙉", "other"},
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(categories); i += 2 {
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(categories[i][0], categories[i][1]),
			tgbotapi.NewInlineKeyboardButtonData(categories[i+1][0], categories[i+1][1]),
		)
		rows = append(rows, row)
	}

	buttons := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, "Выберите категорию расхода:")
	msg.ReplyMarkup = buttons
	h.bot.Send(msg)
}

func (h *BotHandler) addIncome(chatID int64, category string) {
	amount := h.userAmounts[chatID]
	if err := h.service.AddIncome(chatID, amount, category); err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении дохода."))
		log.Printf("Ошибка добавления дохода: %v", err)
	} else {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Доход успешно добавлен."))
	}
	h.resetState(chatID)
	h.sendMainMenu(chatID)
}

func (h *BotHandler) addExpense(chatID int64, category string) {
	amount := h.userAmounts[chatID]
	if err := h.service.AddExpense(chatID, amount, category); err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расхода."))
		log.Printf("Ошибка добавления расхода: %v", err)
	} else {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Расход успешно добавлен."))
	}
	h.resetState(chatID)
	h.sendMainMenu(chatID)
}

// Сброс состояния пользователя
func (h *BotHandler) resetState(chatID int64) {
	h.userStates[chatID] = StateNone
	delete(h.userAmounts, chatID)
}

// Получение отчета
func (h *BotHandler) handleReportCommand(chatID int64) {
	report, err := h.service.GetReport(chatID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении отчета."))
		log.Printf("Ошибка при получении отчета: %v", err)
	} else {
		h.bot.Send(tgbotapi.NewMessage(chatID, report))
	}
}

// package handlers

// import (
// 	"database/sql"
// 	"finuchet-bot/internal/repository"
// 	"finuchet-bot/internal/services"
// 	"log"
// 	"strconv"
// 	"strings"

// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// )

// type BotHandler struct {
// 	bot         *tgbotapi.BotAPI
// 	service     *services.FinanceService
// 	userStates  map[int64]string  // Состояние пользователя
// 	userAmounts map[int64]float64 // Временное хранение суммы для пользователя
// }

// const (
// 	StateNone            = ""
// 	StateWaitingIncome   = "waiting_income"   // Состояние ожидания суммы для дохода
// 	StateWaitingExpense  = "waiting_expense"  // Состояние ожидания суммы для расхода
// 	StateIncomeCategory  = "income_category"  // Состояние ожидания категории дохода
// 	StateExpenseCategory = "expense_category" // Состояние ожидания категории расхода
// )

// func NewBotHandler(token string, db *sql.DB) (*BotHandler, error) {
// 	bot, err := tgbotapi.NewBotAPI(token)
// 	if err != nil {
// 		return nil, err
// 	}

// 	repo := repository.NewPostgresRepository(db)
// 	service := services.NewFinanceService(repo)

// 	return &BotHandler{
// 		bot:         bot,
// 		service:     service,
// 		userStates:  make(map[int64]string),
// 		userAmounts: make(map[int64]float64),
// 	}, nil
// }

// func (h *BotHandler) Start() {
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 60

// 	updates := h.bot.GetUpdatesChan(u)

// 	for update := range updates {
// 		if update.CallbackQuery != nil {
// 			h.handleCallbackQuery(update.CallbackQuery)
// 		}
// 		// if update.Message != nil {
// 		// 	h.handleMessage(update.Message)
// 		// } else if update.CallbackQuery != nil {
// 		// 	h.handleCallbackQuery(update.CallbackQuery)
// 		// }
// 	}
// }

// // Для обработки KeyboardButton:
// // func (h *BotHandler) handleMessage(msg *tgbotapi.Message) {
// // 	chatID := msg.Chat.ID
// // 	text := msg.Text

// // 	// Проверка на групповое сообщение и упоминание бота
// // 	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
// // 		// Убедимся, что бот упомянут в сообщении
// // 		// if !strings.Contains(text, "@"+h.bot.Self.UserName) {
// // 		// 	log.Printf("Игнорируем сообщения без упоминания бота")
// // 		// 	return // Игнорируем сообщения без упоминания бота
// // 		// }
// // 		// Убираем упоминание из текста
// // 		text = strings.ReplaceAll(text, "@"+h.bot.Self.UserName, "")
// // 		text = strings.TrimSpace(text) // Убираем лишние пробелы после удаления упоминания
// // 	}

// // 	switch {
// // 	case text == "/start":
// // 		if err := h.service.RegisterUser(chatID); err != nil {
// // 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при регистрации, попробуйте позже."))
// // 			log.Printf("Ошибка регистрации пользователя: %v", err)
// // 		} else {
// // 			h.sendMainMenu(chatID) // Отправка главного меню с кнопками
// // 		}

// // 	case text == "Доход 📈":
// // 		h.userStates[chatID] = StateWaitingIncome
// // 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму дохода:"))

// // 	case text == "Расход 📉":
// // 		h.userStates[chatID] = StateWaitingExpense
// // 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму расхода:"))

// // 	case text == "Отчет 📊":
// // 		h.handleReportCommand(chatID)

// // 	default:
// // 		h.handleTransactionInput(chatID, text)
// // 	}
// // }

// // Для обработки InlineKeyboard:
// func (h *BotHandler) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
// 	chatID := callbackQuery.Message.Chat.ID
// 	data := callbackQuery.Data

// 	switch data {
// 	case "income":
// 		h.userStates[chatID] = StateWaitingIncome
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму дохода:"))

// 	case "expense":
// 		h.userStates[chatID] = StateWaitingExpense
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму расхода:"))

// 	case "report":
// 		h.handleReportCommand(chatID)
// 	}

// 	// Отметим callback как обработанный
// 	// h.bot.AnswerCallbackQuery(tgbotapi.NewCallback(callbackQuery.ID, ""))
// 	callbackConfig := tgbotapi.NewCallback(callbackQuery.ID, "")
// 	if _, err := h.bot.Request(callbackConfig); err != nil {
// 		log.Printf("Ошибка при отправке CallbackQuery ответа: %v", err)
// 	}
// }

// Отправка главного меню с кнопками "Доход", "Расход" и "Отчет"
// func (h *BotHandler) sendMainMenu(chatID int64) {
// 	buttons := tgbotapi.NewInlineKeyboardMarkup(
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("Доход 📈", "income"),
// 			tgbotapi.NewInlineKeyboardButtonData("Расход 📉", "expense"),
// 		),
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("Отчет 📊", "report"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// func (h *BotHandler) sendMainMenu(chatID int64) {
// 	buttons := tgbotapi.NewReplyKeyboard(
// 		tgbotapi.NewKeyboardButtonRow(
// 			tgbotapi.NewKeyboardButton("Доход 📈"),
// 			tgbotapi.NewKeyboardButton("Расход 📉"),
// 		),
// 		tgbotapi.NewKeyboardButtonRow(
// 			tgbotapi.NewKeyboardButton("Отчет 📊"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// // Отправка кнопок категорий для доходов
// func (h *BotHandler) sendIncomeCategories(chatID int64) {
// 	buttons := tgbotapi.NewInlineKeyboardMarkup(
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("З/п 💸", "salary"),
// 			tgbotapi.NewInlineKeyboardButtonData("Дебитор 🫴", "debit"),
// 		),
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("Инвест 💹", "invest"),
// 			tgbotapi.NewInlineKeyboardButtonData("Вклад 🏦", "deposit"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите категорию дохода:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// // Отправка кнопок категорий для расходов
// func (h *BotHandler) sendExpenseCategories(chatID int64) {
// 	// buttons := tgbotapi.NewInlineKeyboardMarkup(
// 	// 	tgbotapi.NewInlineKeyboardRow(
// 	// 		tgbotapi.NewInlineKeyboardButtonData("Продукты 🛒", "eat"),
// 	// 		tgbotapi.NewInlineKeyboardButtonData("ЖКХ 👾", "room"),
// 	// 		tgbotapi.NewInlineKeyboardButtonData("Кафе 🍜", "cafe"),
// 	// 	),
// 	// 	tgbotapi.NewInlineKeyboardRow(
// 	// 		tgbotapi.NewInlineKeyboardButtonData("Связь 🌐", "link"),
// 	// 		tgbotapi.NewInlineKeyboardButtonData("Образование 📚", "edu"),
// 	// 	),
// 	// )
// 	buttons := tgbotapi.NewInlineKeyboardMarkup(
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("Продукты 🛒", "shop"),
// 			tgbotapi.NewInlineKeyboardButtonData("ЖКХ 👾", "service"),
// 			tgbotapi.NewInlineKeyboardButtonData("Кафе 🍜", "cafe"),
// 		),
// 		tgbotapi.NewInlineKeyboardRow(
// 			tgbotapi.NewInlineKeyboardButtonData("Связь 🌐", "link"),
// 			tgbotapi.NewInlineKeyboardButtonData("Образование 📚", "educ"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите категорию расхода:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// // Обработка ввода данных в зависимости от состояния
// func (h *BotHandler) handleTransactionInput(chatID int64, text string) {
// 	currentState := h.userStates[chatID]
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 60

// 	updates := h.bot.GetUpdatesChan(u)

// 	switch currentState {
// 	case StateWaitingIncome, StateWaitingExpense:
// 		// Проверка и сохранение введенной суммы
// 		amount, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
// 		if err != nil || amount <= 0 {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
// 			return
// 		}

// 		h.userAmounts[chatID] = amount // Сохраняем сумму
// 		if currentState == StateWaitingIncome {
// 			h.userStates[chatID] = StateIncomeCategory
// 			h.sendIncomeCategories(chatID) // Отправка кнопок категорий доходов
// 		} else {
// 			h.userStates[chatID] = StateExpenseCategory
// 			h.sendExpenseCategories(chatID) // Отправка кнопок категорий расходов
// 		}

// 	case StateIncomeCategory:
// 		category := strings.TrimSpace(text)

// 		if category == "" {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную категорию дохода."))
// 			return
// 		}

// 		amount := h.userAmounts[chatID]
// 		if err := h.service.AddIncome(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении дохода."))
// 			log.Printf("Ошибка добавления дохода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Доход успешно добавлен."))
// 		}

// 		// Сброс состояния
// 		h.userStates[chatID] = StateNone
// 		delete(h.userAmounts, chatID) // Удаление временной суммы
// 		// h.sendMainMenu(chatID)        // Возвращаемся к главному меню

// 	case StateExpenseCategory:
// 		category := strings.TrimSpace(text)
// 		if category == "" {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную категорию расхода."))
// 			return
// 		}

// 		amount := h.userAmounts[chatID]
// 		if err := h.service.AddExpense(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расхода."))
// 			log.Printf("Ошибка добавления расхода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Расход успешно добавлен."))
// 		}

// 		// Сброс состояния
// 		h.userStates[chatID] = StateNone
// 		delete(h.userAmounts, chatID) // Удаление временной суммы
// 		// h.sendMainMenu(chatID)        // Возвращаемся к главному меню

// 	default:
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Сначала выберите действие с помощью кнопок "Доход 📈", "Расход 📉" или "Отчет 📊"."))
// 	}

// 	for update := range updates {
// 		if update.CallbackQuery != nil {
// 			h.handleCallbackQuery(update.CallbackQuery) // Возвращаемся к главному меню
// 		}
// 	}
// }

// // Получение ошибки
// func (h *BotHandler) handleReportCommand(chatID int64) {
// 	report, err := h.service.GetReport(chatID)
// 	if err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении отчета."))
// 		log.Printf("Ошибка при получении отчета: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, report))
// 	}
// }

// package handlers

// import (
// 	"database/sql"
// 	"finuchet-bot/internal/repository"
// 	"finuchet-bot/internal/services"
// 	"log"
// 	"strconv"
// 	"strings"

// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// )

// type BotHandler struct {
// 	bot         *tgbotapi.BotAPI
// 	service     *services.FinanceService
// 	userStates  map[int64]string  // Состояние пользователя
// 	userAmounts map[int64]float64 // Временное хранение суммы для пользователя
// }

// const (
// 	StateNone            = ""
// 	StateWaitingIncome   = "waiting_income"   // Состояние ожидания суммы для дохода
// 	StateWaitingExpense  = "waiting_expense"  // Состояние ожидания суммы для расхода
// 	StateIncomeCategory  = "income_category"  // Состояние ожидания категории дохода
// 	StateExpenseCategory = "expense_category" // Состояние ожидания категории расхода
// )

// func NewBotHandler(token string, db *sql.DB) (*BotHandler, error) {
// 	bot, err := tgbotapi.NewBotAPI(token)
// 	if err != nil {
// 		return nil, err
// 	}

// 	repo := repository.NewPostgresRepository(db)
// 	service := services.NewFinanceService(repo)

// 	return &BotHandler{
// 		bot:         bot,
// 		service:     service,
// 		userStates:  make(map[int64]string),
// 		userAmounts: make(map[int64]float64),
// 	}, nil
// }

// func (h *BotHandler) Start() {
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 60

// 	updates := h.bot.GetUpdatesChan(u)

// 	for update := range updates {
// 		if update.Message != nil {
// 			h.handleMessage(update.Message)
// 		}
// 	}
// }

// func (h *BotHandler) handleMessage(msg *tgbotapi.Message) {
// 	chatID := msg.Chat.ID
// 	text := msg.Text

// 	switch {
// 	case text == "/start":
// 		if err := h.service.RegisterUser(chatID); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при регистрации, попробуйте позже."))
// 			log.Printf("Ошибка регистрации пользователя: %v", err)
// 		} else {
// 			h.sendMainMenu(chatID) // Отправка главного меню с кнопками
// 		}

// 	case text == "income":
// 		h.userStates[chatID] = StateWaitingIncome
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму дохода:"))

// 	case text == "expense":
// 		h.userStates[chatID] = StateWaitingExpense
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму расхода:"))

// 	case text == "/report":
// 		h.handleReportCommand(chatID)

// 	default:
// 		h.handleTransactionInput(chatID, text)
// 	}
// }

// // Отправка главного меню с кнопками "income" и "expense"
// func (h *BotHandler) sendMainMenu(chatID int64) {
// 	buttons := tgbotapi.NewReplyKeyboard(
// 		tgbotapi.NewKeyboardButtonRow(
// 			tgbotapi.NewKeyboardButton("income"),
// 			tgbotapi.NewKeyboardButton("expense"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// // Обработка ввода данных в зависимости от состояния
// func (h *BotHandler) handleTransactionInput(chatID int64, text string) {
// 	currentState := h.userStates[chatID]

// 	switch currentState {
// 	case StateWaitingIncome, StateWaitingExpense:
// 		// Проверка и сохранение введенной суммы
// 		amount, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
// 		if err != nil || amount <= 0 {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
// 			return
// 		}

// 		h.userAmounts[chatID] = amount // Сохраняем сумму
// 		if currentState == StateWaitingIncome {
// 			h.userStates[chatID] = StateIncomeCategory
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Введите категорию дохода:"))
// 		} else {
// 			h.userStates[chatID] = StateExpenseCategory
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Введите категорию расхода:"))
// 		}

// 	case StateIncomeCategory:
// 		category := strings.TrimSpace(text)
// 		if category == "" {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную категорию дохода."))
// 			return
// 		}

// 		amount := h.userAmounts[chatID]
// 		if err := h.service.AddIncome(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении дохода."))
// 			log.Printf("Ошибка добавления дохода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Доход успешно добавлен."))
// 		}

// 		// Сброс состояния
// 		h.userStates[chatID] = StateNone
// 		delete(h.userAmounts, chatID) // Удаление временной суммы

// 	case StateExpenseCategory:
// 		category := strings.TrimSpace(text)
// 		if category == "" {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную категорию расхода."))
// 			return
// 		}

// 		amount := h.userAmounts[chatID]
// 		if err := h.service.AddExpense(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расхода."))
// 			log.Printf("Ошибка добавления расхода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Расход успешно добавлен."))
// 		}

// 		// Сброс состояния
// 		h.userStates[chatID] = StateNone
// 		delete(h.userAmounts, chatID) // Удаление временной суммы

// 	default:
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Сначала выберите действие с помощью кнопок "income" или "expense"."))
// 	}
// }

// func (h *BotHandler) handleReportCommand(chatID int64) {
// 	report, err := h.service.GetReport(chatID)
// 	if err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении отчета."))
// 		log.Printf("Ошибка при получении отчета: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, report))
// 	}
// }

// package handlers

// import (
// 	"database/sql"
// 	"finuchet-bot/internal/repository"
// 	"finuchet-bot/internal/services"
// 	"log"
// 	"strconv"
// 	"strings"

// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// )

// type BotHandler struct {
// 	bot        *tgbotapi.BotAPI
// 	service    *services.FinanceService
// 	userStates map[int64]string // Хранит состояние (контекст) для каждого пользователя
// }

// const (
// 	StateNone    = ""
// 	StateIncome  = "income"
// 	StateExpense = "expense"
// )

// func NewBotHandler(token string, db *sql.DB) (*BotHandler, error) {
// 	bot, err := tgbotapi.NewBotAPI(token)
// 	if err != nil {
// 		return nil, err
// 	}

// 	repo := repository.NewPostgresRepository(db)
// 	service := services.NewFinanceService(repo)

// 	return &BotHandler{
// 		bot:        bot,
// 		service:    service,
// 		userStates: make(map[int64]string),
// 	}, nil
// }

// func (h *BotHandler) Start() {
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 60

// 	updates := h.bot.GetUpdatesChan(u)

// 	for update := range updates {
// 		if update.Message != nil {
// 			h.handleMessage(update.Message)
// 		}
// 	}
// }

// func (h *BotHandler) handleMessage(msg *tgbotapi.Message) {
// 	chatID := msg.Chat.ID
// 	text := msg.Text

// 	switch {
// 	case text == "/start":
// 		if err := h.service.RegisterUser(chatID); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при регистрации, попробуйте позже."))
// 			log.Printf("Ошибка регистрации пользователя: %v", err)
// 		} else {
// 			h.sendMainMenu(chatID) // Отправка главного меню с кнопками
// 		}

// 	case text == "income":
// 		h.userStates[chatID] = StateIncome
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму и категорию дохода в формате: <сумма> <категория>"))

// 	case text == "expense":
// 		h.userStates[chatID] = StateExpense
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму и категорию расхода в формате: <сумма> <категория>"))

// 	case text == "/report":
// 		h.handleReportCommand(chatID)

// 	default:
// 		h.handleTransactionInput(chatID, text)
// 	}
// }

// // Отправка главного меню с кнопками "income" и "expense"
// func (h *BotHandler) sendMainMenu(chatID int64) {
// 	buttons := tgbotapi.NewReplyKeyboard(
// 		tgbotapi.NewKeyboardButtonRow(
// 			tgbotapi.NewKeyboardButton("income"),
// 			tgbotapi.NewKeyboardButton("expense"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// // Обработка ввода данных о доходах или расходах в зависимости от состояния
// func (h *BotHandler) handleTransactionInput(chatID int64, text string) {
// 	parts := strings.Fields(text)
// 	if len(parts) != 2 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Неверный формат. Используйте формат: <сумма> <категория>"))
// 		return
// 	}

// 	amount, err := strconv.ParseFloat(parts[0], 64)
// 	if err != nil || amount <= 0 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
// 		return
// 	}

// 	category := parts[1]
// 	currentState := h.userStates[chatID]

// 	switch currentState {
// 	case StateIncome:
// 		if err := h.service.AddIncome(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении дохода."))
// 			log.Printf("Ошибка добавления дохода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Доход успешно добавлен."))
// 		}

// 	case StateExpense:
// 		if err := h.service.AddExpense(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расхода."))
// 			log.Printf("Ошибка добавления расхода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Расход успешно добавлен."))
// 		}
// 	default:
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Сначала выберите действие с помощью кнопок "income" или "expense"."))
// 	}

// 	// Сброс состояния после добавления транзакции
// 	h.userStates[chatID] = StateNone
// }

// func (h *BotHandler) handleReportCommand(chatID int64) {
// 	report, err := h.service.GetReport(chatID)
// 	if err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении отчета."))
// 		log.Printf("Ошибка при получении отчета: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, report))
// 	}
// }

// package handlers

// import (
// 	"database/sql"
// 	"finuchet-bot/internal/repository"
// 	"finuchet-bot/internal/services"
// 	"log"
// 	"strconv"
// 	"strings"

// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// )

// type BotHandler struct {
// 	bot        *tgbotapi.BotAPI
// 	service    *services.FinanceService
// 	userStates map[int64]string // Хранит состояние (контекст) для каждого пользователя
// }

// const (
// 	StateNone    = ""
// 	StateIncome  = "income"
// 	StateExpense = "expense"
// )

// func NewBotHandler(token string, db *sql.DB) (*BotHandler, error) {
// 	bot, err := tgbotapi.NewBotAPI(token)
// 	if err != nil {
// 		return nil, err
// 	}

// 	repo := repository.NewPostgresRepository(db)
// 	service := services.NewFinanceService(repo)

// 	return &BotHandler{
// 		bot:        bot,
// 		service:    service,
// 		userStates: make(map[int64]string),
// 	}, nil
// }

// func (h *BotHandler) Start() {
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 60

// 	updates := h.bot.GetUpdatesChan(u)

// 	for update := range updates {
// 		if update.Message != nil {
// 			h.handleMessage(update.Message)
// 		}
// 	}
// }

// func (h *BotHandler) handleMessage(msg *tgbotapi.Message) {
// 	chatID := msg.Chat.ID
// 	text := msg.Text

// 	switch {
// 	case text == "/start":
// 		if err := h.service.RegisterUser(chatID); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при регистрации, попробуйте позже."))
// 			log.Printf("Ошибка регистрации пользователя: %v", err)
// 		} else {
// 			h.sendMainMenu(chatID) // Отправка главного меню с кнопками
// 		}

// 	case text == "Доход":
// 		h.userStates[chatID] = StateIncome
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму и категорию дохода в формате: <сумма> <категория>"))

// 	case text == "Расход":
// 		h.userStates[chatID] = StateExpense
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Введите сумму и категорию расхода в формате: <сумма> <категория>"))

// 	case text == "/report":
// 		h.handleReportCommand(chatID)

// 	default:
// 		h.handleTransactionInput(chatID, text)
// 	}
// }

// // Отправка главного меню с кнопками "Доход" и "Расход"
// func (h *BotHandler) sendMainMenu(chatID int64) {
// 	buttons := tgbotapi.NewReplyKeyboard(
// 		tgbotapi.NewKeyboardButtonRow(
// 			tgbotapi.NewKeyboardButton("Доход"),
// 			tgbotapi.NewKeyboardButton("Расход"),
// 		),
// 	)

// 	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
// 	msg.ReplyMarkup = buttons
// 	h.bot.Send(msg)
// }

// // Обработка ввода данных о доходах или расходах в зависимости от состояния
// func (h *BotHandler) handleTransactionInput(chatID int64, text string) {
// 	parts := strings.Fields(text)
// 	if len(parts) != 2 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Неверный формат. Используйте формат: <сумма> <категория>"))
// 		return
// 	}

// 	amount, err := strconv.ParseFloat(parts[0], 64)
// 	if err != nil || amount <= 0 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
// 		return
// 	}

// 	category := parts[1]
// 	currentState := h.userStates[chatID]

// 	switch currentState {
// 	case StateIncome:
// 		if err := h.service.AddIncome(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении дохода."))
// 			log.Printf("Ошибка добавления дохода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Доход успешно добавлен."))
// 		}

// 	case StateExpense:
// 		if err := h.service.AddExpense(chatID, amount, category); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расхода."))
// 			log.Printf("Ошибка добавления расхода: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Расход успешно добавлен."))
// 		}
// 	default:
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Сначала выберите действие с помощью кнопок "income" или "expense"."))
// 	}

// 	// Сброс состояния после добавления транзакции
// 	h.userStates[chatID] = StateNone
// }

// func (h *BotHandler) handleReportCommand(chatID int64) {
// 	report, err := h.service.GetReport(chatID)
// 	if err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении отчета."))
// 		log.Printf("Ошибка при получении отчета: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, report))
// 	}
// }

// package handlers

// import (
// 	"database/sql"
// 	"finance-bot/internal/services"
// 	"fmt"
// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// 	"log"
// )

// type BotHandler struct {
// 	bot      *tgbotapi.BotAPI
// 	service  *services.FinanceService
// }

// func NewBotHandler(token string, db *sql.DB) (*BotHandler, error) {
// 	bot, err := tgbotapi.NewBotAPI(token)
// 	if err != nil {
// 		return nil, err
// 	}

// 	repo := repository.NewPostgresRepository(db)
// 	service := services.NewFinanceService(repo)

// 	return &BotHandler{bot: bot, service: service}, nil
// }

// func (h *BotHandler) Start() {
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 60

// 	updates := h.bot.GetUpdatesChan(u)
// 	for update := range updates {
// 		if update.Message != nil {
// 			h.handleMessage(update.Message)
// 		}
// 	}
// }

// func (h *BotHandler) handleMessage(msg *tgbotapi.Message) {
// 	chatID := msg.Chat.ID
// 	text := msg.Text

// 	switch {
// 	case text == "/start":
// 		if err := h.service.RegisterUser(chatID); err != nil {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при регистрации, попробуйте позже."))
// 			log.Printf("Ошибка регистрации пользователя: %v", err)
// 		} else {
// 			h.bot.Send(tgbotapi.NewMessage(chatID, "Добро пожаловать! Используйте команды /income, /expense или /report для работы с ботом."))
// 		}

// 	case strings.HasPrefix(text, "/income"):
// 		h.handleIncomeCommand(chatID, text)

// 	case strings.HasPrefix(text, "/expense"):
// 		h.handleExpenseCommand(chatID, text)

// 	case text == "/report":
// 		h.handleReportCommand(chatID)

// 	default:
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Неизвестная команда. Используйте /income, /expense или /report."))
// 	}
// }

// func (h *BotHandler) handleIncomeCommand(chatID int64, text string) {
// 	parts := strings.Fields(text)
// 	if len(parts) != 3 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Используйте формат: /income <сумма> <категория>"))
// 		return
// 	}

// 	amount, err := strconv.ParseFloat(parts[1], 64)
// 	if err != nil || amount <= 0 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
// 		return
// 	}

// 	category := parts[2]
// 	if err := h.service.AddIncome(chatID, amount, category); err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении дохода."))
// 		log.Printf("Ошибка добавления дохода: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Доход успешно добавлен."))
// 	}
// }

// func (h *BotHandler) handleExpenseCommand(chatID int64, text string) {
// 	parts := strings.Fields(text)
// 	if len(parts) != 3 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Используйте формат: /expense <сумма> <категория>"))
// 		return
// 	}

// 	amount, err := strconv.ParseFloat(parts[1], 64)
// 	if err != nil || amount <= 0 {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Укажите корректную сумму."))
// 		return
// 	}

// 	category := parts[2]
// 	if err := h.service.AddExpense(chatID, amount, category); err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при добавлении расхода."))
// 		log.Printf("Ошибка добавления расхода: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Расход успешно добавлен."))
// 	}
// }

// func (h *BotHandler) handleReportCommand(chatID int64) {
// 	report, err := h.service.GetReport(chatID)
// 	if err != nil {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, "Ошибка при получении отчета."))
// 		log.Printf("Ошибка при получении отчета: %v", err)
// 	} else {
// 		h.bot.Send(tgbotapi.NewMessage(chatID, report))
// 	}
// }
