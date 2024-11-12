# Location Management System

## Introduction

This project is a Location Management System composed of two microservices: `location-history` and `location-management`. The system provides functionalities to update user locations, search for users within a radius, and calculate the distance traveled by a user within a specific time frame.


## Prerequisites

- Go 1.18 or later
- Protocol Buffers (protoc) version 3.0.0 or later
- Git
- PostgreSQL

## Setup

### 1. Clone the repository

```sh
git clone https://github.com/abotoiGrid/Golang-Project.git
cd golang-project
```
### 2. Install the dependencies
```sh
go work init
go work sync

cd db
go mod tidy
cd ..

cd location-history
go mod tidy
cd ..

cd location-management
go mod tidy
cd ..

protoc --go_out=. --go-grpc_out=. location.proto
```
## Run the program

It requires to run two terminals, one to run location-history and the other to run location-management.

```sh
cd location-history
go run main.go
```
The service will start on port: '50051'.
```sh
cd location-management
go run main.go
```
The service will start on port: '8080'.

## API Endpoints
# 1. Update location
    - URL: grpcurl -d '{"username": "testuser", "latitude": 12.345, "longitude": 67.890, "timestamp": "1617188765"}' -plaintext localhost:50051 location.LocationService/UpdateLocation
    - Method: 'POST'
    - Request body:
        {
            "username":"testuser",
            "latitude": 12.345,
            "longitude": 67.890
        }
    - Response:
        {
            "status": "Success"
        }

# 2. Search users
    - URL: curl -X GET "http://localhost:8080/users/search?latitude=35.12314&longitude=27.64532&radius=100&page=1&page_size=10"
    - Method: 'GET'
    - Query parameters:
        - latitude: Latitude of the center point.
        - longitude: Longitude of the center point.
        - radius: Search radius in kilometers.
        - page: Page number (default is 1).
        - size: Number of results per page (default is 10).
    - Response :
        {
            {"total":7,"users":[{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"testuser"},{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"testuser1"},{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"john_doe"},{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"john_doe"},{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"testuser1"},{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"testuser"},{"distance":0,"latitude":35.12314,"longitude":27.64532,"username":"testuser"}]}
        }
# 3. Get distance
    - URL: curl -G "http://localhost:8080/users/distance" --data-urlencode "username=testuser" --data-urlencode "start=2024-11-10T09:00:00Z" --data-urlencode "end=2024-11-10T15:00:00Z"
    - Method: 'GET'
    - Query parameters:
        - 'username': Username of the user
        - 'start_time': Start time in ISO 8601 format
        - 'end_time': End time in ISO 8601 format
    - Response:
        {
            "distance":0,"end":"2024-11-10T15:00:00Z","start":"2024-11-10T09:00:00Z","unit":"kilometers","username":"testuser"
        }