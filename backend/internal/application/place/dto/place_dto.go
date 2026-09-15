package dto

import "github.com/google/uuid"

// NearbyPlaceInfo is the public representation of a place in discovery results.
type NearbyPlaceInfo struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	Address        string    `json:"address"`
	Category       string    `json:"category"`
	ImageURL       string    `json:"image_url"`
	DistanceMeters int64     `json:"distance_meters"`
}

// NearbyPlacesResponse is the response envelope for nearby discovery.
type NearbyPlacesResponse struct {
	Data []NearbyPlaceInfo `json:"data"`
}
