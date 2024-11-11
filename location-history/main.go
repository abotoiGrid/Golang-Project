package main

import (
	"math"
	"net/http"
	"regexp"
	"time"

	"github.com/abotoiGrid/Golang-Project/db"
	"github.com/gin-gonic/gin"
)

func isValidUsername(username string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9]{4,16}$`, username)
	return match
}

func isValidCoordinate(coordinate float64) bool {
	return coordinate >= -180 && coordinate <= 180
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

	router := gin.Default()
	router.GET("/users/distance", CalculateTravelDistance)
	router.Run(":9090")
}
