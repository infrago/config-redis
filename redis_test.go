package config_redis

import (
	"testing"
	"time"

	base "github.com/infrago/base"
	"github.com/infrago/config"
)

func TestDecodeYamlConfig(t *testing.T) {
	cfg, err := config.Decode([]byte("name: demo\n"), "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg["name"] != "demo" {
		t.Fatalf("name=%v, want demo", cfg["name"])
	}
}

func TestDetectYamlFormat(t *testing.T) {
	if got := config.DetectFormat([]byte("name: demo\n")); got != "yaml" {
		t.Fatalf("format=%q, want yaml", got)
	}
}

func TestRedisAddr(t *testing.T) {
	cases := []struct {
		name   string
		params base.Map
		addr   string
	}{
		{name: "default", params: base.Map{}, addr: "127.0.0.1:6379"},
		{name: "addr", params: base.Map{"addr": "redis:6380"}, addr: "redis:6380"},
		{name: "host port", params: base.Map{"host": "redis", "port": "6380"}, addr: "redis:6380"},
		{name: "server with port", params: base.Map{"server": "redis:6380"}, addr: "redis:6380"},
		{name: "server explicit port", params: base.Map{"server": "redis", "port": "6380"}, addr: "redis:6380"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redisAddr(tc.params); got != tc.addr {
				t.Fatalf("addr=%q, want %q", got, tc.addr)
			}
		})
	}
}

func TestRedisMode(t *testing.T) {
	cases := []struct {
		name   string
		params base.Map
		mode   string
	}{
		{name: "default", params: base.Map{}, mode: "standalone"},
		{name: "explicit cluster", params: base.Map{"mode": "cluster"}, mode: "cluster"},
		{name: "cluster addrs", params: base.Map{"cluster_addrs": "a:6379,b:6379"}, mode: "cluster"},
		{name: "sentinel master", params: base.Map{"master_name": "main"}, mode: "sentinel"},
		{name: "sentinel flag", params: base.Map{"sentinel": "true"}, mode: "sentinel"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redisMode(tc.params); got != tc.mode {
				t.Fatalf("mode=%q, want %q", got, tc.mode)
			}
		})
	}
}

func TestRedisAddrs(t *testing.T) {
	got := redisAddrs(base.Map{"cluster_addrs": "redis-a:6379, redis-b:6379"}, []string{"cluster_addrs"}, "fallback:6379")
	want := []string{"redis-a:6379", "redis-b:6379"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("addrs=%v, want %v", got, want)
	}
}

func TestRedisTLSConfig(t *testing.T) {
	if cfg := redisTLSConfig(base.Map{}); cfg != nil {
		t.Fatalf("tls config=%v, want nil", cfg)
	}
	cfg := redisTLSConfig(base.Map{
		"tls":                      "true",
		"tls_server_name":          "redis.internal",
		"tls_insecure_skip_verify": "true",
	})
	if cfg == nil {
		t.Fatal("tls config is nil")
	}
	if cfg.ServerName != "redis.internal" {
		t.Fatalf("serverName=%q, want redis.internal", cfg.ServerName)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify")
	}
}

func TestRedisTimeout(t *testing.T) {
	if got := redisTimeout("250ms"); got != 250*time.Millisecond {
		t.Fatalf("timeout=%v, want 250ms", got)
	}
	if got := redisTimeout("2"); got != 2*time.Second {
		t.Fatalf("timeout=%v, want 2s", got)
	}
	if got := redisTimeout(nil); got != 3*time.Second {
		t.Fatalf("timeout=%v, want 3s", got)
	}
}
