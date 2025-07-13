package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

func GetConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	if err = json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Override with environment variables
	if port := os.Getenv("PORT"); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		cfg.App.Port = p
	}

	// DB environment variables
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Repository.DBHost = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Repository.DBPort = p
		}
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.Repository.DBUsername = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.Repository.DBPassword = password
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		cfg.Repository.DBName = name
	}

	// Redis environment variables (ДОБАВЛЕНО)
	if redisHost := os.Getenv("REDIS_HOST"); redisHost != "" {
		cfg.Cache.RedisHost = redisHost
	}
	if redisPort := os.Getenv("REDIS_PORT"); redisPort != "" {
		cfg.Cache.RedisPort, _ = strconv.Atoi(redisPort)
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		cfg.Cache.RedisPassword = redisPassword
	}
	if redisDB := os.Getenv("REDIS_DB"); redisDB != "" {
		cfg.Cache.RedisDB, _ = strconv.Atoi(redisDB)
	}
	if poolSize := os.Getenv("REDIS_POOL_SIZE"); poolSize != "" {
		cfg.Cache.PoolSize, _ = strconv.Atoi(poolSize)
	}
	if minIdleConns := os.Getenv("REDIS_MIN_IDLE_CONNS"); minIdleConns != "" {
		cfg.Cache.MinIdleConns, _ = strconv.Atoi(minIdleConns)
	}
	if dialTimeout := os.Getenv("REDIS_DIAL_TIMEOUT"); dialTimeout != "" {
		cfg.Cache.DialTimeout = dialTimeout
	}
	if readTimeout := os.Getenv("REDIS_READ_TIMEOUT"); readTimeout != "" {
		cfg.Cache.ReadTimeout = readTimeout
	}
	if writeTimeout := os.Getenv("REDIS_WRITE_TIMEOUT"); writeTimeout != "" {
		cfg.Cache.WriteTimeout = writeTimeout
	}

	// Exchange environment variables
	if len(cfg.Exchanges.LiveExchanges) >= 3 {
		if exchange1Host := os.Getenv("EXCHANGE1_HOST"); exchange1Host != "" {
			cfg.Exchanges.LiveExchanges[0].Host = exchange1Host
		}
		if exchange1Port := os.Getenv("EXCHANGE1_PORT"); exchange1Port != "" {
			if p, err := strconv.Atoi(exchange1Port); err == nil {
				cfg.Exchanges.LiveExchanges[0].Port = p
			}
		}
		if exchange2Host := os.Getenv("EXCHANGE2_HOST"); exchange2Host != "" {
			cfg.Exchanges.LiveExchanges[1].Host = exchange2Host
		}
		if exchange2Port := os.Getenv("EXCHANGE2_PORT"); exchange2Port != "" {
			if p, err := strconv.Atoi(exchange2Port); err == nil {
				cfg.Exchanges.LiveExchanges[1].Port = p
			}
		}
		if exchange3Host := os.Getenv("EXCHANGE3_HOST"); exchange3Host != "" {
			cfg.Exchanges.LiveExchanges[2].Host = exchange3Host
		}
		if exchange3Port := os.Getenv("EXCHANGE3_PORT"); exchange3Port != "" {
			if p, err := strconv.Atoi(exchange3Port); err == nil {
				cfg.Exchanges.LiveExchanges[2].Port = p
			}
		}
	}

	// Test mode environment variables
	if updateInterval := os.Getenv("TEST_UPDATE_INTERVAL_MS"); updateInterval != "" {
		if interval, err := strconv.Atoi(updateInterval); err == nil {
			cfg.Exchanges.TestMode.UpdateIntervalMs = interval
		}
	}

	return &cfg, nil
}

type Config struct {
	App        App        `json:"app"`
	Repository Repository `json:"repository"`
	Cache      Cache      `json:"cache"`
	Exchanges  Exchanges  `json:"exchanges"`
}

type App struct {
	Port int `json:"port"`
}

type Repository struct {
	DBHost      string `json:"db_host"`
	DBPort      int    `json:"db_port"`
	DBUsername  string `json:"db_username"`
	DBPassword  string `json:"db_password"`
	DBName      string `json:"db_name"`
	DBSSLMode   string `json:"db_ssl_mode"`
	MaxConn     int    `json:"max_conn"`
	MaxIdleConn int    `json:"max_idle_conn"`
}

type Cache struct {
	RedisHost     string `json:"redis_host"`
	RedisPort     int    `json:"redis_port"`
	RedisPassword string `json:"redis_password"`
	RedisDB       int    `json:"redis_db"`
	PoolSize      int    `json:"pool_size"`
	MinIdleConns  int    `json:"min_idle_conns"`
	DialTimeout   string `json:"dial_timeout"`
	ReadTimeout   string `json:"read_timeout"`
	WriteTimeout  string `json:"write_timeout"`
}

type Exchanges struct {
	LiveExchanges []ExchangeConfig `json:"live_exchanges"`
	TestMode      TestModeConfig   `json:"test_mode"`
}

type ExchangeConfig struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TestModeConfig struct {
	UpdateIntervalMs int      `json:"update_interval_ms"`
	Symbols          []string `json:"symbols"`
}
