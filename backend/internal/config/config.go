package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server         ServerConfig
	Postgres       PostgresConfig
	Redis          RedisConfig
	JWT            JWTConfig
	SSH            SSHConfig
	GitHub         GitHubConfig
	Registry       RegistryConfig
	Log            LogConfig
	Security       SecurityConfig
	SelfServer     SelfServerConfig
	Admin          AdminConfig
	MigrationsPath string
}

type AdminConfig struct {
	Email    string
	Password string
}

type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret        string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type SSHConfig struct {
	HostKeyPath     string
	DefaultPort     int
	ConnectionTTL   time.Duration
	MaxConnections  int
}

type GitHubConfig struct {
	Token string
}

type RegistryConfig struct {
	URL             string
	ExternalURL     string
	AdminUser       string
	AdminPass       string
	HtpasswdPath    string
	ZotContainer    string
}

type SecurityConfig struct {
	RateLimitMaxAttempts  int           `json:"rate_limit_max_attempts"`
	RateLimitWindow       time.Duration `json:"rate_limit_window"`
	RateLimitLockout      time.Duration `json:"rate_limit_lockout"`
	MinPasswordLength     int           `json:"min_password_length"`
}

type SelfServerConfig struct {
	Enabled         bool   // SELF_SERVER_ENABLED — auto-register host server
	Name            string // SELF_SERVER_NAME — display name for self server
	DockerSocketPath string // DOCKER_SOCKET_PATH — path to Docker socket
	HostNetwork     string // SELF_HOST_NETWORK — host IP from inside container (e.g. 172.22.0.1)
}

type LogConfig struct {
	Level string
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode,
	)
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:  getDurationEnv("SERVER_IDLE_TIMEOUT", 60*time.Second),
		},
		Postgres: PostgresConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", "anjungan"),
			Password: getEnv("POSTGRES_PASSWORD", "anjungan"),
			DBName:   getEnv("POSTGRES_DB", "anjungan"),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", "change-me-in-production"),
			AccessTTL:  getDurationEnv("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: getDurationEnv("JWT_REFRESH_TTL", 7*24*time.Hour),
		},
		SSH: SSHConfig{
			HostKeyPath:    getEnv("SSH_HOST_KEY_PATH", "/data/ssh/id_ed25519"),
			DefaultPort:    getEnvInt("SSH_DEFAULT_PORT", 22),
			ConnectionTTL:  getDurationEnv("SSH_CONNECTION_TTL", 30*time.Minute),
		},
		GitHub: GitHubConfig{
			Token: getEnv("GITHUB_TOKEN", ""),
		},
		Registry: RegistryConfig{
			URL:             getEnv("REGISTRY_URL", "http://zot:5000"),
			ExternalURL:     getEnv("REGISTRY_EXTERNAL_URL", "registry.anjungan.io"),
			AdminUser:       getEnv("ZOT_ADMIN_USER", "admin"),
			AdminPass:       getEnv("ZOT_ADMIN_PASS", "z0t_4dm1n_p4ss"),
			HtpasswdPath:    getEnv("ZOT_HTPASSWD_PATH", "/data/zot/htpasswd"),
			ZotContainer:    getEnv("ZOT_CONTAINER_NAME", "anjungan-zot"),
		},
		Security: SecurityConfig{
			RateLimitMaxAttempts:  getEnvInt("RATE_LIMIT_MAX_ATTEMPTS", 5),
			RateLimitWindow:       getDurationEnv("RATE_LIMIT_WINDOW", 15*time.Minute),
			RateLimitLockout:      getDurationEnv("RATE_LIMIT_LOCKOUT", 30*time.Minute),
			MinPasswordLength:     getEnvInt("MIN_PASSWORD_LENGTH", 8),
		},
		SelfServer: SelfServerConfig{
			Enabled:          getEnvBool("SELF_SERVER_ENABLED", false),
			Name:             getEnv("SELF_SERVER_NAME", "anjungan-host"),
			DockerSocketPath: getEnv("DOCKER_SOCKET_PATH", "/var/run/docker.sock"),
			HostNetwork:      getEnv("SELF_HOST_NETWORK", ""),
		},
		Admin: AdminConfig{
			Email:    getEnv("ADMIN_EMAIL", ""),
			Password: getEnv("ADMIN_PASSWORD", ""),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		MigrationsPath: getEnv("MIGRATIONS_PATH", "/migrations"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val == "true" || val == "1" || val == "yes"
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
