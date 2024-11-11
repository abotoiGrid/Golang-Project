package main

import (
	"context"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/abotoiGrid/Golang-Project/db"
	pb "github.com/abotoiGrid/Golang-Project/proto"
	"google.golang.org/grpc"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var locationHistoryClient pb.locationHistoryClient

func initGRPCClient() {
	conn, err := grpc.Dial("microservice2-address:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to LocationHistory service: %v", err)
	}
	locationHistoryClient = pb.NewLocationHistoryClient(conn)
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

	_, err = locationHistoryClient.SaveLocation(context.Background(), &pb.LocationRequest{
		Username:  request.Username,
		Latitude:  request.Latitude,
		Longitute: request.Longitude,
		Timestamp: time.Now().Unix(),
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

func main() {
	db.InitDB()
	defer db.DB.Close()
	initGRPCClient()

	router := gin.Default()
	router.POST("/location/update", UpdateLocation)
	router.GET("/users/search", searchUsers)
	router.Run(":8080")
}
