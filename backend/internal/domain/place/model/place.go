package model

import (
	"time"

	"github.com/google/uuid"
)

// Place represents an active or inactive place discoverable in NIMNear.
type Place struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Address     string    `json:"address"`
	Category    string    `json:"category"`
	ImageURL    string    `json:"image_url"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NearbyPlace is a place enriched with its distance from a search point.
type NearbyPlace struct {
	Place
	DistanceMeters float64 `json:"-"`
}
