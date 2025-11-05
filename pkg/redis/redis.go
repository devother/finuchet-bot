package redis

// var ctx = context.Background()

// // RedisClient - глобальный клиент Redis
// var RedisClient *redis.Client

// // InitializeRedis - настройка и подключение к Redis
// func InitializeRedis(addr, password string, db int) {
// 	RedisClient = redis.NewClient(&redis.Options{
// 		Addr:     addr,     // адрес Redis (например, "localhost:6379")
// 		Password: password, // пароль, если требуется (оставьте пустым, если нет)
// 		DB:       db,       // номер базы данных
// 	})

// 	// Проверка соединения
// 	_, err := RedisClient.Ping(ctx).Result()
// 	if err != nil {
// 		log.Fatalf("Не удалось подключиться к Redis: %v", err)
// 	}

// 	log.Println("Успешно подключен к Redis")
// }

// // SetValue - пример записи значения в Redis
// func SetValue(key string, value interface{}, expiration time.Duration) error {
// 	return RedisClient.Set(ctx, key, value, expiration).Err()
// }

// // GetValue - пример получения значения из Redis
// func GetValue(key string) (string, error) {
// 	return RedisClient.Get(ctx, key).Result()
// }
