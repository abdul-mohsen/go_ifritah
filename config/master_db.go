package config

import (
	"database/sql"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// MasterDB is the live connection to the deployment-owned tenant metadata DB.
var MasterDB *sql.DB

func initializeMasterDB() {
	dsn := strings.TrimSpace(os.Getenv("MASTER_DB_DSN"))
	if dsn == "" || MasterDB != nil {
		return
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("⚠️  Master DB disabled: could not open MASTER_DB_DSN: %v", err)
		return
	}
	MasterDB = db
}
