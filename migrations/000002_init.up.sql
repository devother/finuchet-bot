----------------------------------------------------
CREATE TABLE user_categories (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES users(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL,
    type VARCHAR(10) CHECK (type IN ('income', 'expense')) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, category, type) -- Уникальность названия категории и типа для пользователя
);
----------------------------------------------------
-- Удаляем старое текстовое поле category и добавляем внешний ключ, 
-- который будет ссылаться на новую таблицу user_categories.
ALTER TABLE transactions
DROP COLUMN category;

ALTER TABLE transactions
ADD COLUMN category_id INT REFERENCES user_categories(id) ON DELETE SET NULL;