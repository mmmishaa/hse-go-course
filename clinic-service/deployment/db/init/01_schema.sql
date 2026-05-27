-- Создается только если volume пустой
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Таблицы создаются в repository.go при старте,
-- но можно продублировать здесь для initdb