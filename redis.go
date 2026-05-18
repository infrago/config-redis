package config_redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	base "github.com/infrago/base"
	"github.com/infrago/config"
	"github.com/infrago/infra"

	"github.com/redis/go-redis/v9"
)

type redisConfigDriver struct{}

type redisConfigClient interface {
	Get(context.Context, string) *redis.StringCmd
	Close() error
}

func init() {
	infra.Register("redis", &redisConfigDriver{})
}

func (d *redisConfigDriver) Load(params base.Map) (base.Map, error) {
	addr := redisAddr(params)
	username, _ := params["username"].(string)
	password, _ := params["password"].(string)
	timeout := redisTimeout(params["timeout"])

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
	if vv, ok := params["key"].(string); ok && strings.TrimSpace(vv) != "" {
		key = strings.TrimSpace(vv)
	}

	format := ""
	if vv, ok := params["format"].(string); ok {
		format = strings.TrimSpace(vv)
	}

	client, target, err := newRedisClient(params, addr, username, password, db, timeout)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	val, err := client.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis get key %q from %q failed: %w", key, target, err)
	}

	if format == "" {
		format = config.DetectFormat([]byte(val))
	}

	return config.Decode([]byte(val), format)
}

func newRedisClient(params base.Map, addr, username, password string, db int, timeout time.Duration) (redisConfigClient, string, error) {
	mode := redisMode(params)
	tlsConfig := redisTLSConfig(params)

	switch mode {
	case "cluster":
		addrs := redisAddrs(params, []string{"cluster_addrs", "addrs", "addr"}, addr)
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:                 addrs,
			Username:              username,
			Password:              password,
			DialTimeout:           timeout,
			ReadTimeout:           timeout,
			WriteTimeout:          timeout,
			ContextTimeoutEnabled: true,
			TLSConfig:             tlsConfig,
			RouteByLatency:        boolParam(params["route_by_latency"]),
			RouteRandomly:         boolParam(params["route_randomly"]),
			ReadOnly:              boolParam(params["read_only"]),
			DisableIdentity:       true,
			FailingTimeoutSeconds: intParam(params["failing_timeout_seconds"]),
		}), "cluster:" + strings.Join(addrs, ","), nil
	case "sentinel", "failover":
		master := firstString(params, "master", "master_name", "sentinel_master")
		if master == "" {
			return nil, "", fmt.Errorf("redis sentinel master name is required")
		}
		addrs := redisAddrs(params, []string{"sentinel_addrs", "sentinels", "addrs"}, addr)
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:            master,
			SentinelAddrs:         addrs,
			SentinelUsername:      firstString(params, "sentinel_username"),
			SentinelPassword:      firstString(params, "sentinel_password"),
			Username:              username,
			Password:              password,
			DB:                    db,
			DialTimeout:           timeout,
			ReadTimeout:           timeout,
			WriteTimeout:          timeout,
			ContextTimeoutEnabled: true,
			TLSConfig:             tlsConfig,
			RouteByLatency:        boolParam(params["route_by_latency"]),
			RouteRandomly:         boolParam(params["route_randomly"]),
			ReplicaOnly:           boolParam(params["replica_only"]),
			DisableIdentity:       true,
		}), "sentinel:" + master + "@" + strings.Join(addrs, ","), nil
	default:
		return redis.NewClient(&redis.Options{
			Addr:                  addr,
			Username:              username,
			Password:              password,
			DB:                    db,
			DialTimeout:           timeout,
			ReadTimeout:           timeout,
			WriteTimeout:          timeout,
			ContextTimeoutEnabled: true,
			TLSConfig:             tlsConfig,
			DisableIdentity:       true,
		}), addr, nil
	}
}

func redisMode(params base.Map) string {
	mode := strings.ToLower(firstString(params, "mode", "redis_mode"))
	switch mode {
	case "cluster", "sentinel", "failover":
		return mode
	}
	if boolParam(params["cluster"]) || len(stringList(params["cluster_addrs"])) > 0 {
		return "cluster"
	}
	if boolParam(params["sentinel"]) || firstString(params, "master", "master_name", "sentinel_master") != "" || len(stringList(params["sentinel_addrs"])) > 0 {
		return "sentinel"
	}
	return "standalone"
}

func redisAddr(params base.Map) string {
	if addr, ok := params["addr"].(string); ok && strings.TrimSpace(addr) != "" {
		return strings.TrimSpace(addr)
	}

	host := ""
	if v, ok := params["server"].(string); ok && strings.TrimSpace(v) != "" {
		host = strings.TrimSpace(v)
	}
	if v, ok := params["host"].(string); ok && strings.TrimSpace(v) != "" {
		host = strings.TrimSpace(v)
	}
	if host == "" {
		host = "127.0.0.1"
	}

	port := "6379"
	if v, ok := params["port"].(string); ok && strings.TrimSpace(v) != "" {
		port = strings.TrimSpace(v)
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, port)
}

func redisAddrs(params base.Map, keys []string, fallback string) []string {
	for _, key := range keys {
		if values := stringList(params[key]); len(values) > 0 {
			return values
		}
	}
	return []string{fallback}
}

func redisTLSConfig(params base.Map) *tls.Config {
	enabled := boolParam(params["tls"]) || boolParam(params["tls_enabled"])
	serverName := firstString(params, "tls_server_name", "server_name")
	insecure := boolParam(params["tls_insecure_skip_verify"]) || boolParam(params["tls_skip_verify"]) || boolParam(params["insecure_skip_verify"])
	if !enabled && serverName == "" && !insecure {
		return nil
	}
	return &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: insecure,
		MinVersion:         tls.VersionTLS12,
	}
}

func redisTimeout(value base.Any) time.Duration {
	const fallback = 3 * time.Second
	switch v := value.(type) {
	case time.Duration:
		if v > 0 {
			return v
		}
	case int:
		if v > 0 {
			return time.Duration(v) * time.Second
		}
	case int64:
		if v > 0 {
			return time.Duration(v) * time.Second
		}
	case float64:
		if v > 0 {
			return time.Duration(v * float64(time.Second))
		}
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return fallback
		}
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return time.Duration(n * float64(time.Second))
		}
	}
	return fallback
}

func firstString(params base.Map, keys ...string) string {
	for _, key := range keys {
		if v, ok := params[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func stringList(value base.Any) []string {
	switch v := value.(type) {
	case []string:
		return cleanStrings(v)
	case []base.Any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return cleanStrings(out)
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return cleanStrings(strings.Split(v, ","))
	default:
		return nil
	}
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func boolParam(value base.Any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		}
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	}
	return false
}

func intParam(value base.Any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}
