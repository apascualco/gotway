package persistence

import (
	"errors"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type UserEntity struct {
	Email     string    `json:"user"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func (userEntity *UserEntity) Create() error {
	db := GetSqlitle3Connection()

	statement, err := db.Prepare(`INSERT INTO User(email, password, created_at, updated_at) values(?, ?, ?, ?)`)
	if err != nil {
		return err
	}

	defer statement.Close()
	result, err := statement.Exec(userEntity.Email, userEntity.Password, time.Now(), time.Now())
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected != 1 {
		return errors.New("Error creating new user")
	}
	return nil
}

func (userEntity *UserEntity) GetByEmail(email string) error {
	db := GetSqlitle3Connection()
	query := `SELECT email, password, created_at, updated_at FROM User user WHERE user.email=$1`
	row := db.QueryRow(query, email)
	return row.Scan(&userEntity.Email, &userEntity.Password, &userEntity.CreatedAt, &userEntity.UpdatedAt)
}
