package repository

import (
	"database/sql"
	"finuchet-bot/internal/models"
)

type Repository interface {
	GetUserByChatID(chatID int64) (*models.User, error)
	CreateUser(user *models.User) error
	AddTransaction(transaction *models.Transaction) error
	DelData(chatID int64) error
	GetTransactions(userID int64) ([]*models.Transaction, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) Repository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) GetUserByChatID(chatID int64) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow("SELECT id, chat_id FROM users WHERE chat_id = $1", chatID).Scan(&user.ID, &user.ChatID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

func (r *PostgresRepository) CreateUser(user *models.User) error {
	_, err := r.db.Exec("INSERT INTO users (chat_id) VALUES ($1) ON CONFLICT DO NOTHING", user.ChatID)
	return err
}

func (r *PostgresRepository) AddTransaction(transaction *models.Transaction) error {
	_, err := r.db.Exec("WITH ins0 AS (
							INSERT INTO user_categories (user_id, category, type)
							VALUES ($1, 'book', 'expense')
							RETURNING id, type
							)
							INSERT INTO transactions (category_id, type, user_id, amount)
							SELECT id, type, '1', 543
							FROM   ins0 ON CONFLICT DO NOTHING
	INSERT INTO transactions (user_id, amount, category, type) VALUES ($1, $2, $3, $4)",
		transaction.UserID, transaction.Amount, transaction.Category, transaction.Type)
	return err
}

func (r *PostgresRepository) DelData(userID int64) error {
	_, err := r.db.Exec("DELETE FROM transactions WHERE user_id = $1", userID)
	return err
}

func (r *PostgresRepository) GetTransactions(userID int64) ([]*models.Transaction, error) {
	rows, err := r.db.Query("SELECT id, user_id, amount, category, type, created_at FROM transactions WHERE user_id = $1", userID)
	// rows, err := r.db.Query("SELECT tr.id, tr.user_id, tr.amount, uc.category, uc.type, tr.create_at FROM transactions as tr JOIN user_categories as uc ON uc.user_id = tr.user_id WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		transaction := &models.Transaction{}
		err = rows.Scan(&transaction.ID, &transaction.UserID, &transaction.Amount, &transaction.Category, &transaction.Type, &transaction.CreatedAt)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}
