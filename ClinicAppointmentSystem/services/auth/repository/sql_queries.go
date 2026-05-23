package repository

const (
	createUserQuery = `
        INSERT INTO users (id, login, password_hash, full_name, phone, email, role)
        VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
        RETURNING id, created_at
    `
	getUserByLoginQuery = `
        SELECT id, login, password_hash, full_name, phone, email, role, is_active, created_at, updated_at
        FROM users
        WHERE login = $1
    `
	getUserByIDQuery = `
        SELECT id, login, full_name, phone, email, role, is_active, created_at, updated_at
        FROM users
        WHERE id = $1
    `
	createSessionQuery = `
        INSERT INTO sessions (session_id, user_id, expires_at)
        VALUES ($1, $2, NOW() + INTERVAL '24 hours')
    `
	getSessionByUserIDQuery = `
        SELECT session_id, user_id, created_at, expires_at
        FROM sessions
        WHERE user_id = $1 AND expires_at > NOW()
    `
	updateSessionExpiryQuery = `
        UPDATE sessions
        SET expires_at = NOW() + INTERVAL '24 hours'
        WHERE session_id = $1
    `
	deleteSessionQuery = `
        DELETE FROM sessions
        WHERE session_id = $1
    `
)
