package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func UpdateLocation(c *gin.Context) {
	var request struct {
		Username  string  `json:"username" binding:"required"`
		Latitude  float64 `json:"latitude" binding:"required"`
		Longitude float64 `json:"longitude" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := DB.Exec("INSERT INTO user_locations (username, latitude, longitude) VALUES ($1, $2, $3)",
		request.Username, request.Latitude, request.Longitude)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "location updated"})
}

func main() {
	InitDB()
	defer DB.Close()

	router := gin.Default()
	router.POST("/location/update", UpdateLocation)
	router.Run(":8080")
}
