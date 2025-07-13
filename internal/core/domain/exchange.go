package domain

type MarketData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
	Exchange  string  // Add exchange name to identify source
}
