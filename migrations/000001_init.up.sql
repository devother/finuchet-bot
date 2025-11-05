----------------------------------------------------
-- Таблицы:
-- Создаем users
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- Создаем transactions
CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(10, 2) NOT NULL,
    category VARCHAR(50) NOT NULL,
    type VARCHAR(10) CHECK (type IN ('income', 'expense')) NOT NULL,
    create_dat DATE DEFAULT CURRENT_DATE,
    updated_at DATE DEFAULT CURRENT_DATE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
----------------------------------------------------
-- Функции
-- Создаем функцию для обновления поля updated_at
CREATE OR REPLACE FUNCTION update_date()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_DATE;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
----------------------------------------------------
-- Триггеры
-- Создаем триггер, который срабатывает при обновлении записи
CREATE TRIGGER set_update_date
BEFORE UPDATE ON transactions
FOR EACH ROW
EXECUTE FUNCTION update_date();