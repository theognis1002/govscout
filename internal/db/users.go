package db

import "database/sql"

type UserRow struct {
	ID           int64
	Username     string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    string
}

func CreateUser(db *sql.DB, username, passwordHash string, isAdmin bool) error {
	admin := 0
	if isAdmin {
		admin = 1
	}
	_, err := db.Exec("INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)",
		username, passwordHash, admin)
	return err
}

func UpdatePassword(db *sql.DB, username, passwordHash string) error {
	res, err := db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", passwordHash, username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func GetUserByUsername(db *sql.DB, username string) (*UserRow, error) {
	var u UserRow
	var admin int
	err := db.QueryRow("SELECT id, username, password_hash, is_admin, created_at FROM users WHERE username = ?",
		username).Scan(&u.ID, &u.Username, &u.PasswordHash, &admin, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.IsAdmin = admin == 1
	return &u, nil
}

func ListUsers(db *sql.DB) ([]UserRow, error) {
	rows, err := db.Query("SELECT id, username, is_admin, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserRow
	for rows.Next() {
		var u UserRow
		var admin int
		if err := rows.Scan(&u.ID, &u.Username, &admin, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.IsAdmin = admin == 1
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func DeleteUser(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}
