package config_redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/infrago/infra"
	base "github.com/infrago/base"
	"github.com/infrago/config"

	"github.com/pelletier/go-toml/v2"
	"github.com/redis/go-redis/v9"
)

type redisConfigDriver struct{}

func init() {
	infra.Register("redis", &redisConfigDriver{})
}

func (d *redisConfigDriver) Load(params base.Map) (base.Map, error) {
	server, _ := params["server"].(string)
	port, _ := params["port"].(string)
	username, _ := params["username"].(string)
	password, _ := params["password"].(string)

	addr := "127.0.0.1:6379"
	if server != "" {
		if port != "" {
			addr = server + ":" + port
		} else {
			addr = server + ":6379"
		}
	}

	db := 0
	switch v := params["database"].(type) {
	case int:
		db = v
	case int64:
		db = int(v)
	case float64:
		db = int(v)
	case string:
		if vv, err := strconv.Atoi(v); err == nil {
			db = vv
		}
	}

	key := config.KEY
	if vv, ok := params["key"].(string); ok {
		key = vv
	}

	format := ""
	if vv, ok := params["format"].(string); ok {
		format = vv
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: username,
		Password: password,
		DB:       db,
	})
	defer client.Close()

	val, err := client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis get failed: %w", err)
	}

	if format == "" {
		format = detectFormat([]byte(val))
	}

	return decodeConfig([]byte(val), format)
}

func decodeConfig(data []byte, format string) (base.Map, error) {
	var out base.Map
	switch strings.ToLower(format) {
	case "json":
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	case "toml":
		if err := toml.Unmarshal(data, &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, errors.New("Unknown config format: " + format)
	}
}

func detectFormat(data []byte) string {
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return "json"
	}
	return "toml"
}
