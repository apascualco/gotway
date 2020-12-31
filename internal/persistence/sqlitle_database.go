package persistence

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func GetSqlitle3Connection() *sql.DB {
	if db != nil {
		return db
	}

	db, err := sql.Open("sqlite3", "data.sqlite")
	if err != nil {
		log.Print("Error ")
		log.Println(err)
		return nil
	}

	return db
}

func LaunchDDL() {
	db := GetSqlitle3Connection()
	q := `CREATE TABLE IF NOT EXISTS User (
		email VARCHAR(64) PRIMARY KEY,
		password VARCHAR(200) NULL,
		created_at TIMESTAMP DEFAULT DATETIME,
		updated_at TIMESTAMP NOT NULL
	);`
	db.Exec(q)
}
