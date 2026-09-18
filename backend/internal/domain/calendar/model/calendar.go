package model

import (
	"time"

	"github.com/google/uuid"
)

// Visibility controls whether a calendar can be discovered publicly.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// Status controls the lifecycle of a calendar.
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// Calendar is a server-owned event collection.
type Calendar struct {
	ID          uuid.UUID
	Name        string
	Description string
	ImageURL    string
	OwnerID     uuid.UUID
	Visibility  Visibility
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Patch contains explicit calendar field updates.
type Patch struct {
	NameSet        bool
	Name           string
	DescriptionSet bool
	Description    string
	ImageURLSet    bool
	ImageURL       string
	VisibilitySet  bool
	Visibility     Visibility
}
