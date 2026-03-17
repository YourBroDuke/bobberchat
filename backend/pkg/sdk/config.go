package sdk

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	defaultHeartbeatInterval = 30 * time.Second
	defaultRequestTimeout    = 30 * time.Second
)

func LoadConfig(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetEnvPrefix("BOBBERCHAT")
	v.AutomaticEnv()

	_ = v.BindEnv("backend_url", "BOBBERCHAT_BACKEND_URL")
	_ = v.BindEnv("agent_id", "BOBBERCHAT_AGENT_ID")
	_ = v.BindEnv("api_secret", "BOBBERCHAT_API_SECRET")
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := Config{
		BackendURL:   strings.TrimSpace(v.GetString("backend_url")),
		AgentID:      strings.TrimSpace(v.GetString("agent_id")),
		APISecret:    strings.TrimSpace(v.GetString("api_secret")),
		DisplayName:  strings.TrimSpace(v.GetString("display_name")),
		Capabilities: v.GetStringSlice("capabilities"),

		HeartbeatInterval: defaultHeartbeatInterval,
		RequestTimeout:    defaultRequestTimeout,
	}

	heartbeatMS := v.GetInt("heartbeat_interval_ms")
	if heartbeatMS > 0 {
		cfg.HeartbeatInterval = time.Duration(heartbeatMS) * time.Millisecond
	}

	requestTimeoutMS := v.GetInt("request_timeout_ms")
	if requestTimeoutMS > 0 {
		cfg.RequestTimeout = time.Duration(requestTimeoutMS) * time.Millisecond
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func LoadConfigFromEnv() Config {
	cfg := Config{
		BackendURL:        strings.TrimSpace(os.Getenv("BOBBERCHAT_BACKEND_URL")),
		AgentID:           strings.TrimSpace(os.Getenv("BOBBERCHAT_AGENT_ID")),
		APISecret:         strings.TrimSpace(os.Getenv("BOBBERCHAT_API_SECRET")),
		DisplayName:       strings.TrimSpace(os.Getenv("BOBBERCHAT_DISPLAY_NAME")),
		Capabilities:      csvListEnv("BOBBERCHAT_CAPABILITIES"),
		HeartbeatInterval: defaultHeartbeatInterval,
		RequestTimeout:    defaultRequestTimeout,
	}

	if ms, ok := envPositiveInt("BOBBERCHAT_HEARTBEAT_INTERVAL_MS"); ok {
		cfg.HeartbeatInterval = time.Duration(ms) * time.Millisecond
	}

	if ms, ok := envPositiveInt("BOBBERCHAT_REQUEST_TIMEOUT_MS"); ok {
		cfg.RequestTimeout = time.Duration(ms) * time.Millisecond
	}

	return cfg
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.BackendURL) == "" {
		return errors.New("backend_url is required")
	}

	if strings.TrimSpace(cfg.AgentID) == "" {
		return errors.New("agent_id is required")
	}

	if strings.TrimSpace(cfg.APISecret) == "" {
		return errors.New("api_secret is required")
	}

	if cfg.HeartbeatInterval <= 0 {
		return errors.New("heartbeat_interval must be positive")
	}

	if cfg.RequestTimeout <= 0 {
		return errors.New("request_timeout must be positive")
	}

	return nil
}

func envPositiveInt(key string) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, false
	}

	return v, true
}

func csvListEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}
