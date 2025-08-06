package domain

type Prices struct {
	PairName     string  `json:"pair_name"`
	Exchange     string  `json:"exchange"`
	Timestamp    int64   `json:"timestamp"`
	AveragePrice float64 `json:"average_price"`
	MinPrice     float64 `json:"min_price"`
	MaxPrice     float64 `json:"max_price"`
}

type GetPrice struct {
	Price int
}
