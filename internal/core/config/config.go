package config

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	AppEnv                string
	AllowedOrigins        []string
	RateLimitRPS          float64
	RateLimitBurst        int
	WsMaxPayloadBytes     int64
	DbMaxConns            int
	DbMinConns            int
	DbQueryTimeoutSeconds int
	DbTxTimeoutSeconds    int
	JWTAccessSecret       string
	JWTRefreshSecret      string
	AccessTokenTTLMinutes int
	RefreshTokenTTLDays   int
	RefreshCookieName     string
	RefreshCookieSecure   bool
	RefreshCookieDomain   string
	RefreshCookiePath     string
	RefreshCookieSameSite string
}

func LoadConfig() *Config {
	return &Config{
		Port:                  getEnv("PORT", "8000"),
		DatabaseURL:           getEnv("DATABASE_URL", ""),
		AppEnv:                getEnv("APP_ENV", "development"),
		AllowedOrigins:        strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),
		RateLimitRPS:          getEnvFloat("RATE_LIMIT_RPS", 5.0),
		RateLimitBurst:        getEnvInt("RATE_LIMIT_BURST", 10),
		WsMaxPayloadBytes:     int64(getEnvInt("WS_MAX_PAYLOAD_BYTES", 1048576)),
		DbMaxConns:            getEnvInt("DB_MAX_CONNS", 10),
		DbMinConns:            getEnvInt("DB_MIN_CONNS", 2),
		DbQueryTimeoutSeconds: getEnvInt("DB_QUERY_TIMEOUT_SECONDS", 5),
		DbTxTimeoutSeconds:    getEnvInt("DB_TX_TIMEOUT_SECONDS", 10),
		JWTAccessSecret:       getEnv("JWT_ACCESS_SECRET", ""),
		JWTRefreshSecret:      getEnv("JWT_REFRESH_SECRET", ""),
		AccessTokenTTLMinutes: getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 15),
		RefreshTokenTTLDays:   getEnvInt("REFRESH_TOKEN_TTL_DAYS", 30),
		RefreshCookieName:     getEnv("REFRESH_COOKIE_NAME", "refresh_token"),
		RefreshCookieSecure:   getEnvBool("REFRESH_COOKIE_SECURE", false),
		RefreshCookieDomain:   getEnv("REFRESH_COOKIE_DOMAIN", ""),
		RefreshCookiePath:     getEnv("REFRESH_COOKIE_PATH", "/"),
		RefreshCookieSameSite: strings.ToLower(getEnv("REFRESH_COOKIE_SAME_SITE", "lax")),
	}
}

func (c *Config) DBQueryTimeout() time.Duration {
	return time.Duration(c.DbQueryTimeoutSeconds) * time.Second
}

func (c *Config) DBTxTimeout() time.Duration {
	return time.Duration(c.DbTxTimeoutSeconds) * time.Second
}

func (c *Config) AccessTokenTTL() time.Duration {
	return time.Duration(c.AccessTokenTTLMinutes) * time.Minute
}

func (c *Config) RefreshTokenTTL() time.Duration {
	return time.Duration(c.RefreshTokenTTLDays) * 24 * time.Hour
}

func (c *Config) RefreshCookieMaxAge() int {
	return int(c.RefreshTokenTTL().Seconds())
}

func (c *Config) CookieSameSite() http.SameSite {
	switch strings.ToLower(c.RefreshCookieSameSite) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		slog.Warn("Invalid same-site env, using lax", "value", c.RefreshCookieSameSite)
		return http.SameSiteLaxMode
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	value, err := strconv.Atoi(strValue)
	if err != nil {
		slog.Warn("Invalid numeric env, using default", "key", key, "default", fallback)
		return fallback
	}
	return value
}

func getEnvFloat(key string, fallback float64) float64 {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(strValue, 64)
	if err != nil {
		slog.Warn("Invalid float env, using default", "key", key, "default", fallback)
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	strValue := getEnv(key, "")
	if strValue == "" {
		return fallback
	}
	value, err := strconv.ParseBool(strValue)
	if err != nil {
		slog.Warn("Invalid boolean env, using default", "key", key, "default", fallback)
		return fallback
	}
	return value
}
