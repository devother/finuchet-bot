package services

import (
	"finuchet-bot/internal/models"
	"finuchet-bot/internal/repository"
	"fmt"
)

type FinanceService struct {
	repo repository.Repository
}

func NewFinanceService(repo repository.Repository) *FinanceService {
	return &FinanceService{repo: repo}
}

func (s *FinanceService) RegisterUser(chatID int64) error {
	user, err := s.repo.GetUserByChatID(chatID)
	if err != nil || user != nil {
		return err
	}
	return s.repo.CreateUser(&models.User{ChatID: chatID})
}

// Метод обработки доходов
func (s *FinanceService) AddIncome(chatID int64, amount float64, category string) error {
	user, err := s.repo.GetUserByChatID(chatID)
	if err != nil || user == nil {
		return err
	}
	return s.repo.AddTransaction(&models.Transaction{
		UserID:   user.ID,
		Amount:   amount,
		Category: category,
		Type:     "income",
	})
}

// Метод обработки расходов
func (s *FinanceService) AddExpense(chatID int64, amount float64, category string) error {
	user, err := s.repo.GetUserByChatID(chatID)
	if err != nil || user == nil {
		return err
	}
	return s.repo.AddTransaction(&models.Transaction{
		UserID:   user.ID,
		Amount:   amount,
		Category: category,
		Type:     "expense",
	})
}

// Метод очистки данных
func (s *FinanceService) ClearData(chatID int64) error {
	user, err := s.repo.GetUserByChatID(chatID)
	if err != nil || user == nil {
		return err
	}

	return s.repo.DelData(user.ID)
}

func (s *FinanceService) GetReport(chatID int64) (string, error) {
	user, err := s.repo.GetUserByChatID(chatID)
	if err != nil || user == nil {
		return "", err
	}

	transactions, err := s.repo.GetTransactions(user.ID)
	if err != nil {
		return "", err
	}

	var income, expense float64
	for _, t := range transactions {
		if t.Type == "income" {
			income += t.Amount
		} else if t.Type == "expense" {
			expense += t.Amount
		}
	}

	report := "Доходы: " + fmt.Sprintf("%.2f", income) + "\n" +
		"Расходы: " + fmt.Sprintf("%.2f", expense) + "\n" +
		"Баланс: " + fmt.Sprintf("%.2f", income-expense)

	return report, nil
}
