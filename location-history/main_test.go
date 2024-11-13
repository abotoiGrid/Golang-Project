package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/abotoiGrid/Golang-Project/db"
	pb "github.com/abotoiGrid/Golang-Project/proto"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

var testDB *sql.DB

func setupTestDB() {
	var err error
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbUser == "" || dbPassword == "" || dbName == "" {
		log.Fatal("Database credentials are missing!")
	}
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", dbUser, dbPassword, dbName)
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	createTableQuery := `
    CREATE TABLE IF NOT EXISTS user_locations (
        id SERIAL PRIMARY KEY,
        username TEXT,
        latitude REAL,
        longitude REAL,
        timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
	_, err = testDB.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func teardownTestDB() {
	if testDB != nil {
		testDB.Close()
	}
}

func TestUpdateLocation(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()
	db.DB = testDB

	s := &server{db: testDB}

	_, err := s.UpdateLocation(context.Background(), &pb.LocationRequest{
		Username:  "testuser8",
		Latitude:  37.7749,
		Longitude: -122.4194,
		Timestamp: strconv.FormatInt(time.Now().Unix(), 10),
	})

	assert.NoError(t, err)

	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM user_locations WHERE username = $1", "testuser8").Scan(&count)

	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}
