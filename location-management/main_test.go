package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/abotoiGrid/Golang-Project/db"
	pb "github.com/abotoiGrid/Golang-Project/proto"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
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

	err = testDB.Ping()
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	createTableQuery := `
    CREATE TABLE IF NOT EXISTS user_locations (
        username TEXT PRIMARY KEY,
        latitude REAL,
        longitude REAL,
        timestamp TIMESTAMP
    );
    `
	_, err = testDB.Exec(createTableQuery)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
}

type MockLocationServiceClient struct {
	pb.LocationServiceClient
}

func (m *MockLocationServiceClient) UpdateLocation(ctx context.Context, in *pb.LocationRequest, opts ...grpc.CallOption) (*pb.LocationResponse, error) {
	return &pb.LocationResponse{}, nil
}

func setupTestClient() {
	locationHistoryClient = &MockLocationServiceClient{}
}

func teardownTestDB() {
	if testDB != nil {
		testDB.Close()
	}
}

func TestUpdateLocation(t *testing.T) {
	setupTestDB()
	setupTestClient()
	defer teardownTestDB()

	db.DB = testDB

	r := gin.Default()
	r.POST("/location/update", UpdateLocation)

	// Test valid username
	w := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]interface{}{
		"username":  "testuser",
		"latitude":  40.7749,
		"longitude": -120.4194,
	})
	req, _ := http.NewRequest("POST", "/location/update", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "location updated")

	// Test invalid username
	w = httptest.NewRecorder()
	body, _ = json.Marshal(map[string]interface{}{
		"username":  "test@user",
		"latitude":  40.7749,
		"longitude": -120.4194,
	})
	req, _ = http.NewRequest("POST", "/location/update", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")

	// Test invalid latitude
	w = httptest.NewRecorder()
	body, _ = json.Marshal(map[string]interface{}{
		"username":  "testuser",
		"latitude":  100.0, // Invalid latitude
		"longitude": -120.4194,
	})
	req, _ = http.NewRequest("POST", "/location/update", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")

	// Test invalid longitude
	w = httptest.NewRecorder()
	body, _ = json.Marshal(map[string]interface{}{
		"username":  "testuser",
		"latitude":  40.7749,
		"longitude": -200.0, // Invalid longitude
	})
	req, _ = http.NewRequest("POST", "/location/update", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestSearchUsers(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()

	db.DB = testDB

	r := gin.Default()
	r.GET("/users/search", searchUsers)

	// Insert some test data
	testDB.Exec("INSERT INTO user_locations (username, latitude, longitude) VALUES (?, ?, ?)",
		"testuser", 37.7749, -122.4194)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users/search?latitude=37.7749&longitude=-122.4194&radius=1&page=1&page_size=10", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "testuser")
}

func TestCalculateTravelDistance(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()

	db.DB = testDB

	r := gin.Default()
	r.GET("/users/distance", CalculateTravelDistance)

	// Insert some test data
	_, err := testDB.Exec("INSERT INTO user_locations (username, latitude, longitude, timestamp) VALUES ($1, $2, $3, $4)",
		"testuser", 37.7749, -122.4194, "2023-01-01T00:00:00Z")
	assert.NoError(t, err)
	_, err = testDB.Exec("INSERT INTO user_locations (username, latitude, longitude, timestamp) VALUES ($1, $2, $3, $4)",
		"testuser", 37.7750, -122.4195, "2023-01-01T01:00:00Z")
	assert.NoError(t, err)

	// Test valid request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/users/distance?username=testuser&start=2023-01-01T00:00:00Z&end=2023-01-01T02:00:00Z", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "testuser")
	assert.Contains(t, w.Body.String(), "distance")

	// Test invalid username
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/users/distance?username=test@user&start=2023-01-01T00:00:00Z&end=2023-01-01T02:00:00Z", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid username")

	// Test request with no data in the specified time range
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/users/distance?username=testuser&start=2024-01-01T00:00:00Z&end=2024-01-01T02:00:00Z", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "distance")
	assert.Contains(t, w.Body.String(), "\"distance\":0")
}
