package dto

import "github.com/google/uuid"

// PlaceInfo is the safe public representation of an active place.
type PlaceInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Address     string    `json:"address"`
	Category    string    `json:"category"`
	ImageURL    string    `json:"image_url"`
}

// NearbyPlaceInfo is a public place enriched with distance from a request coordinate.
type NearbyPlaceInfo struct {
	PlaceInfo
	DistanceMeters int64 `json:"distance_meters"`
}

// PlaceResponse is the public detail envelope for an active place.
type PlaceResponse struct {
	Data PlaceInfo `json:"data"`
}

// NearbyPlacesResponse is the response envelope for nearby discovery.
type NearbyPlacesResponse struct {
	Data []NearbyPlaceInfo `json:"data"`
}
