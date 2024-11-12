package main

import (
	"context"
	"log"
	"math"
	"net/http"
	"regexp"
	"time"

	"github.com/abotoiGrid/Golang-Project/db"
	pb "github.com/abotoiGrid/Golang-Project/proto"
	"google.golang.org/grpc"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var locationHistoryClient pb.LocationServiceClient

func initGRPCClient() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to LocationHistory service: %v", err)
	}
	locationHistoryClient = pb.NewLocationServiceClient(conn)
}

func UpdateLocation(c *gin.Context) {
	var request struct {
		Username  string  `json:"username" binding:"required,alphanum,min=4,max=16"`
		Latitude  float64 `json:"latitude" binding:"required,gte=-90,lte=90"`
		Longitude float64 `json:"longitude" binding:"required,gte=-180,lte=180"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.DB.Exec("INSERT INTO user_locations (username, latitude, longitude) VALUES ($1, $2, $3)",
		request.Username, request.Latitude, request.Longitude)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	timestamp := time.Now()
	timestampStr := timestamp.Format(time.RFC3339)

	_, err = locationHistoryClient.UpdateLocation(context.Background(), &pb.LocationRequest{
		Username:  request.Username,
		Latitude:  request.Latitude,
		Longitude: request.Longitude,
		Timestamp: timestampStr,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed too communicate with LocationHistory service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "location updated"})
}

func CalculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func searchUsers(c *gin.Context) {
	var request struct {
		Latitude  float64 `form:"latitude" binding:"required,gte=-90,lte=90"`
		Longitude float64 `form:"longitude" binding:"required,gte=-180,lte=180"`
		Radius    float64 `form:"radius" binding:"required"`
		Page      int     `form:"page,default=1"`
		PageSize  int     `form:"page_size,default=10"`
	}

	if err := c.ShouldBindQuery(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	offset := (request.Page - 1) * request.PageSize
	rows, err := db.DB.Query(`
        SELECT username, latitude, longitude 
        FROM user_locations
        WHERE earth_box(ll_to_earth($1, $2), $3) @> ll_to_earth(latitude, longitude)
        LIMIT $4 OFFSET $5`,
		request.Latitude, request.Longitude, request.Radius*1000, request.PageSize, offset)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query database"})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var username string
		var latitude, longitude float64
		if err := rows.Scan(&username, &latitude, &longitude); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan row"})
			return
		}

		distance := CalculateDistance(request.Latitude, request.Longitude, latitude, longitude)
		if distance <= request.Radius {
			results = append(results, map[string]interface{}{
				"username":  username,
				"latitude":  latitude,
				"longitude": longitude,
				"distance":  distance,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"users": results,
		"total": len(results),
	})
}

func isValidUsername(username string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9]{4,16}$`, username)
	return match
}

func isValidCoordinate(coordinate float64) bool {
	return coordinate >= -180 && coordinate <= 180
}

func CalculateTravelDistance(c *gin.Context) {
	var request struct {
		Username string    `form:"username" binding:"required,alphanum,min=4,max=16"`
		Start    time.Time `form:"start" time_format:"2006-01-02T15:04:05Z07:00"`
		End      time.Time `form:"end" time_format:"2006-01-02T15:04:05Z07:00"`
	}

	if err := c.ShouldBindQuery(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Start.IsZero() {
		request.End = time.Now()
		request.Start = request.End.Add(-24 * time.Hour)
	}

	grpcRequest := &pb.TravelDistanceRequest{
		Username: request.Username,
		Start:    request.Start.Format(time.RFC3339),
		End:      request.End.Format(time.RFC3339),
	}

	// Call the gRPC CalculateTravelDistance method
	resp, err := locationHistoryClient.CalculateTravelDistance(context.Background(), grpcRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to communicate with LocationHistory service"})
		log.Printf("gRPC error: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": resp.Username,
		"distance": resp.Distance,
		"unit":     resp.Unit,
		"start":    resp.Start,
		"end":      resp.End,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to communicate with LocationHistory service"})
		log.Printf("gRPC error: %v", err)
		return
	}

	if !isValidUsername(request.Username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid username. Must be 4-16 alphanumeric characters"})
		return
	}

	// Default to last 24 hours if no time range specified
	if request.Start.IsZero() {
		request.End = time.Now()
		request.Start = request.End.Add(-24 * time.Hour)
	}

	rows, err := db.DB.Query(`
        SELECT latitude, longitude, timestamp
        FROM user_locations
        WHERE username = $1 AND timestamp BETWEEN $2 AND $3
        ORDER BY timestamp ASC`,
		request.Username, request.Start, request.End)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query database"})
		return
	}
	defer rows.Close()

	var totalDistance float64
	var prevLat, prevLon float64
	first := true

	for rows.Next() {
		var latitude, longitude float64
		var timestamp time.Time
		if err := rows.Scan(&latitude, &longitude, &timestamp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan row"})
			return
		}

		if !isValidCoordinate(latitude) || !isValidCoordinate(longitude) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coordinates"})
			return
		}

		if !first {
			totalDistance += CalculateDistance(prevLat, prevLon, latitude, longitude)
		} else {
			first = false
		}

		prevLat = latitude
		prevLon = longitude
	}

	c.JSON(http.StatusOK, gin.H{
		"username": request.Username,
		"distance": totalDistance,
		"unit":     "kilometers",
		"start":    request.Start,
		"end":      request.End,
	})
}

func main() {
	db.InitDB()
	defer db.DB.Close()
	initGRPCClient()

	router := gin.Default()
	router.POST("/location/update", UpdateLocation)
	router.GET("/users/search", searchUsers)
	router.GET("/users/distance", CalculateTravelDistance)
	router.Run(":8080")
}
