package domain

import (
	"time"
)

type Prices struct {
	PairName     string    `json:"pair_name"`     // The trading pair name (e.g., BTC/USD)
	Exchange     string    `json:"exchange"`      // Exchange from which the data was received
	Timestamp    time.Time `json:"timestamp"`     // Time when the data is stored
	AveragePrice float64   `json:"average_price"` // Average price over the last minute
	MinPrice     float64   `json:"min_price"`     // Minimum price over the last minute
	MaxPrice     float64   `json:"max_price"`     // Maximum price over the last minute
}

type GetPrice struct {
	Price int
}
