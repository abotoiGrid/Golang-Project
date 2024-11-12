package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"net"
	"strconv"
	"time"

	"github.com/abotoiGrid/Golang-Project/db"
	pb "github.com/abotoiGrid/Golang-Project/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	pb.UnimplementedLocationServiceServer
}

func (s *server) UpdateLocation(ctx context.Context, req *pb.LocationRequest) (*pb.LocationResponse, error) {
	timestamp, err := strconv.ParseInt(req.Timestamp, 10, 64)
	if err != nil {
		return &pb.LocationResponse{Status: "Failed"}, fmt.Errorf("failed to parse timestamp: %v", err)
	}
	timestampTime := time.Unix(timestamp, 0)

	_, err = db.DB.Exec("INSERT INTO user_locations (username, latitude, longitude, timestamp) VALUES ($1, $2, $3, $4)",
		req.Username, req.Latitude, req.Longitude, timestampTime)
	if err != nil {
		return &pb.LocationResponse{Status: "Failed"}, err
	}

	return &pb.LocationResponse{Status: "Success"}, nil
}
func (s *server) CalculateTravelDistance(ctx context.Context, req *pb.TravelDistanceRequest) (*pb.TravelDistanceResponse, error) {
	// Extract username, start, and end from the request
	username := req.GetUsername()
	startTime, err := time.Parse(time.RFC3339, req.GetStart())
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %v", err)
	}
	endTime, err := time.Parse(time.RFC3339, req.GetEnd())
	if err != nil {
		return nil, fmt.Errorf("failed to parse end time: %v", err)
	}

	// Query the database for locations within the given time range
	rows, err := db.DB.Query(`
		SELECT latitude, longitude, timestamp
		FROM user_locations
		WHERE username = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp ASC`, username, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %v", err)
	}
	defer rows.Close()

	var totalDistance float64
	var prevLat, prevLon float64
	first := true

	for rows.Next() {
		var latitude, longitude float64
		var timestamp time.Time
		if err := rows.Scan(&latitude, &longitude, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}

		if !first {
			// Calculate distance between the previous and current location
			totalDistance += CalculateDistance(prevLat, prevLon, latitude, longitude)
		} else {
			first = false
		}

		prevLat = latitude
		prevLon = longitude
	}

	// Return the travel distance response
	return &pb.TravelDistanceResponse{
		Username: username,
		Distance: totalDistance,
		Unit:     "kilometers",
		Start:    req.GetStart(),
		End:      req.GetEnd(),
	}, nil
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

func main() {
	db.InitDB()
	defer db.DB.Close()

	go func() {
		lis, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterLocationServiceServer(s, &server{})
		reflection.Register(s)
		log.Println("LocationHistory gRPC server started on :50051")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

}
