package models

import (
	"time"
)

type User struct {
	ID     int64
	ChatID int64
}

type Transaction struct {
	ID        int64
	UserID    int64
	Amount    float64
	Category  string
	Type      string // "income" или "expense"
	CreatedAt time.Time
}
