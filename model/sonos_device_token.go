package model

import (
	"time"
)

type SonosDeviceToken struct {
	ID          string    `structs:"id" json:"id"`
	UserID      string    `structs:"user_id" json:"userId"`
	HouseholdID string    `structs:"household_id" json:"householdId"`
	Token       string    `structs:"token" json:"token"`
	DeviceName  string    `structs:"device_name" json:"deviceName"`
	LastSeenAt  time.Time `structs:"last_seen_at" json:"lastSeenAt"`
	CreatedAt   time.Time `structs:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `structs:"updated_at" json:"updatedAt"`
}

type SonosDeviceTokens []SonosDeviceToken

type SonosDeviceTokenRepository interface {
	// Get retrieves a token by ID
	Get(id string) (*SonosDeviceToken, error)
	// GetByToken retrieves a token by the token value
	GetByToken(token string) (*SonosDeviceToken, error)
	// GetByUserID retrieves all tokens for a user
	GetByUserID(userID string) (SonosDeviceTokens, error)
	// GetByHouseholdID retrieves all tokens for a household
	GetByHouseholdID(householdID string) (SonosDeviceTokens, error)
	// Put creates or updates a token
	Put(token *SonosDeviceToken) error
	// Delete removes a token by ID
	Delete(id string) error
	// DeleteByUserID removes all tokens for a user
	DeleteByUserID(userID string) error
	// UpdateLastSeen updates the last seen timestamp
	UpdateLastSeen(id string, lastSeen time.Time) error
	// CountAll returns total number of tokens
	CountAll(...QueryOptions) (int64, error)
}
