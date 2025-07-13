package prices

import (
	"errors"
	"log/slog"

	"crypto/internal/core/port"
)

type MarketService struct {
	priceRepository port.PriceRepository
}

func (s *MarketService) GetLatestPrice(symbol string) (int, error) {
	if symbol != "BTCUSDT" || symbol != "DOGEUSDT" || symbol != "TONUSDT" || symbol != "SOLUSDT" || symbol != "ETHUSDT" {
		slog.Error("There is not sumbol with a name: ", symbol)
		return 0, errors.New("There is not sumbol with that name")
	}
	price, err := s.GetLatestPrice(symbol)
	if err != nil {
		slog.Error(err.Error())
		return 0, err
	}
	return price, nil
}

func (s *MarketService) GetLatestPriceByExchange(symbol string, exchange string) (int, error) {
	if symbol != "BTCUSDT" || symbol != "DOGEUSDT" || symbol != "TONUSDT" || symbol != "SOLUSDT" || symbol != "ETHUSDT" {
		slog.Error("There is not sumbol with a name: ", symbol)
		return 0, errors.New("There is not sumbol with that name")
	}
	price, err := s.GetLatestPriceByExchange(symbol, exchange)
	if err != nil {
		slog.Error(err.Error())
		return 0, err
	}
	return price, nil
}
