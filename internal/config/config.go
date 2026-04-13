package config

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

// MkGoVersion is the misskey-go version. Override at build time via:
//
//	go build -ldflags "-X github.com/shiroha-a/mk/internal/config.MkGoVersion=1.0.0"
var MkGoVersion = "0.0.1"

// MisskeyVersion is the compatible Misskey version. Override at build time via:
//
//	go build -ldflags "-X github.com/shiroha-a/mk/internal/config.MisskeyVersion=2026.3.2"
var MisskeyVersion = "2026.3.2"

// RedisOptions represents Redis connection configuration.
type RedisOptions struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Family   int    `mapstructure:"family"`
	Pass     string `mapstructure:"pass"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
	Username string `mapstructure:"username"`
}

// DBOptions represents PostgreSQL connection configuration.
type DBOptions struct {
	Host         string            `mapstructure:"host"`
	Port         int               `mapstructure:"port"`
	DB           string            `mapstructure:"db"`
	User         string            `mapstructure:"user"`
	Pass         string            `mapstructure:"pass"`
	DisableCache bool              `mapstructure:"disableCache"`
	Extra        map[string]string `mapstructure:"extra"`
}

// MeilisearchOptions represents Meilisearch configuration.
type MeilisearchOptions struct {
	Host   string `mapstructure:"host"`
	Port   string `mapstructure:"port"`
	APIKey string `mapstructure:"apiKey"`
	SSL    bool   `mapstructure:"ssl"`
	Index  string `mapstructure:"index"`
	Scope  string `mapstructure:"scope"`
}

// FulltextSearchOptions represents fulltext search configuration.
type FulltextSearchOptions struct {
	Provider string `mapstructure:"provider"`
}

// LoggingOptions represents logging configuration.
type LoggingOptions struct {
	SQL *SQLLoggingOptions `mapstructure:"sql"`
}

// SQLLoggingOptions represents SQL logging configuration.
type SQLLoggingOptions struct {
	DisableQueryTruncation bool `mapstructure:"disableQueryTruncation"`
	EnableQueryParamLog    bool `mapstructure:"enableQueryParamLogging"`
}

// Source represents the raw YAML configuration file structure.
type Source struct {
	URL                    string                 `mapstructure:"url"`
	Port                   int                    `mapstructure:"port"`
	Socket                 string                 `mapstructure:"socket"`
	ChmodSocket            string                 `mapstructure:"chmodSocket"`
	DisableHSTS            bool                   `mapstructure:"disableHsts"`
	EnableIPRateLimit      *bool                  `mapstructure:"enableIpRateLimit"`
	DB                     DBOptions              `mapstructure:"db"`
	DBReplications         bool                   `mapstructure:"dbReplications"`
	Redis                  RedisOptions           `mapstructure:"redis"`
	RedisForPubsub         *RedisOptions          `mapstructure:"redisForPubsub"`
	RedisForJobQueue       *RedisOptions          `mapstructure:"redisForJobQueue"`
	RedisForTimelines      *RedisOptions          `mapstructure:"redisForTimelines"`
	RedisForReactions      *RedisOptions          `mapstructure:"redisForReactions"`
	FulltextSearch         *FulltextSearchOptions `mapstructure:"fulltextSearch"`
	Meilisearch            *MeilisearchOptions    `mapstructure:"meilisearch"`
	SetupPassword          string                 `mapstructure:"setupPassword"`
	Proxy                  string                 `mapstructure:"proxy"`
	ProxySmtp              string                 `mapstructure:"proxySmtp"`
	ProxyBypassHosts       []string               `mapstructure:"proxyBypassHosts"`
	AllowedPrivateNetworks []string               `mapstructure:"allowedPrivateNetworks"`
	MaxFileSize            *int64                 `mapstructure:"maxFileSize"`
	ClusterLimit           *int                   `mapstructure:"clusterLimit"`
	ID                     string                 `mapstructure:"id"`
	OutgoingAddress        string                 `mapstructure:"outgoingAddress"`
	OutgoingAddressFamily  string                 `mapstructure:"outgoingAddressFamily"`

	DeliverJobConcurrency      *int `mapstructure:"deliverJobConcurrency"`
	InboxJobConcurrency        *int `mapstructure:"inboxJobConcurrency"`
	RelationshipJobConcurrency *int `mapstructure:"relationshipJobConcurrency"`
	DeliverJobPerSec           *int `mapstructure:"deliverJobPerSec"`
	InboxJobPerSec             *int `mapstructure:"inboxJobPerSec"`
	RelationshipJobPerSec      *int `mapstructure:"relationshipJobPerSec"`
	DeliverJobMaxAttempts      *int `mapstructure:"deliverJobMaxAttempts"`
	InboxJobMaxAttempts        *int `mapstructure:"inboxJobMaxAttempts"`

	MediaProxy              string          `mapstructure:"mediaProxy"`
	MediaProxySecret        string          `mapstructure:"mediaProxySecret"`
	VideoThumbnailGenerator string          `mapstructure:"videoThumbnailGenerator"`
	Logging                 *LoggingOptions `mapstructure:"logging"`

	PerChannelMaxNoteCacheCount  *int   `mapstructure:"perChannelMaxNoteCacheCount"`
	PerUserNotificationsMaxCount *int   `mapstructure:"perUserNotificationsMaxCount"`
	DeactivateAntennaThreshold   *int   `mapstructure:"deactivateAntennaThreshold"`
	PidFile                      string `mapstructure:"pidFile"`

	// TrustProxy is a list of CIDR ranges for trusted reverse proxies.
	// When set, Echo uses X-Forwarded-For from these ranges to determine
	// the real client IP. Defaults to private IP ranges (TS-compatible).
	TrustProxy []string `mapstructure:"trustProxy"`

	// TestMode enables destructive test-only endpoints such as /api/reset-db.
	// Must never be enabled in production. Can be overridden via MK_TESTMODE=1.
	TestMode bool `mapstructure:"testMode"`
}

// Config represents the resolved application configuration.
type Config struct {
	Version string
	URL     string
	Port    int
	// Socket, if non-empty, is a path to a UNIX domain socket that the HTTP
	// server listens on instead of the TCP port.
	Socket string
	// ChmodSocket is the mode (as a string like "770") applied to the UNIX
	// domain socket file after bind. Ignored when Socket is empty or when
	// the value cannot be parsed as octal.
	ChmodSocket string
	Host        string
	Hostname    string
	Scheme      string
	WsScheme    string
	WsURL       string
	APIURL      string
	AuthURL     string
	DriveURL    string

	DisableHSTS       bool
	EnableIPRateLimit bool
	SetupPassword     string

	DB             DBOptions
	DBReplications bool

	Redis             RedisOptions
	RedisForPubsub    RedisOptions
	RedisForJobQueue  RedisOptions
	RedisForTimelines RedisOptions
	RedisForReactions RedisOptions

	FulltextSearch *FulltextSearchOptions
	Meilisearch    *MeilisearchOptions

	ID string

	Proxy                  string
	ProxySmtp              string
	ProxyBypassHosts       []string
	AllowedPrivateNetworks []string

	MaxFileSize           int64
	ClusterLimit          *int
	OutgoingAddress       string
	OutgoingAddressFamily string

	DeliverJobConcurrency      *int
	InboxJobConcurrency        *int
	RelationshipJobConcurrency *int
	DeliverJobPerSec           *int
	InboxJobPerSec             *int
	RelationshipJobPerSec      *int
	DeliverJobMaxAttempts      *int
	InboxJobMaxAttempts        *int

	MediaProxy                   string
	ExternalMediaProxyEnabled    bool
	MediaProxySecret             []byte
	VideoThumbnailGenerator      string
	UserAgent                    string
	PerChannelMaxNoteCacheCount  int
	PerUserNotificationsMaxCount int
	DeactivateAntennaThreshold   int
	PidFile                      string
	Logging                      *LoggingOptions

	// TrustProxy is a list of CIDR ranges for trusted reverse proxies.
	TrustProxy []string

	// TestMode enables destructive test-only endpoints such as /api/reset-db.
	// Must never be enabled in production. Can be overridden via MK_TESTMODE=1.
	TestMode bool
}

const defaultMaxFileSize int64 = 262144000

// Load reads the Misskey YAML configuration and returns a resolved Config.
// Environment variables with the prefix MK_ override YAML values.
// Nested keys use underscore separation (e.g. MK_DB_HOST overrides db.host).
func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 環境変数によるオーバーライド
	v.SetEnvPrefix("MK")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 主要な設定キーを明示的にバインド（Viperは既知のキーのみ環境変数を適用する）
	bindEnvKeys(v)

	var src Source
	if err := v.Unmarshal(&src); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return resolve(&src)
}

// bindEnvKeys binds environment variables to known configuration keys.
func bindEnvKeys(v *viper.Viper) {
	keys := []string{
		"url", "port", "socket",
		"db.host", "db.port", "db.db", "db.user", "db.pass",
		"redis.host", "redis.port", "redis.pass", "redis.db", "redis.username",
		"redisForPubsub.host", "redisForPubsub.port", "redisForPubsub.pass",
		"redisForJobQueue.host", "redisForJobQueue.port", "redisForJobQueue.pass",
		"redisForTimelines.host", "redisForTimelines.port", "redisForTimelines.pass",
		"redisForReactions.host", "redisForReactions.port", "redisForReactions.pass",
		"id", "maxFileSize",
		"mediaProxySecret",
		"testMode",
	}
	for _, k := range keys {
		_ = v.BindEnv(k)
	}
}

func resolve(src *Source) (*Config, error) {
	parsedURL, err := url.Parse(src.URL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid url: %s", src.URL)
	}

	host := parsedURL.Host
	hostname := parsedURL.Hostname()
	scheme := strings.TrimSuffix(parsedURL.Scheme, ":")
	wsScheme := strings.Replace(scheme, "http", "ws", 1)

	maxFileSize := defaultMaxFileSize
	if src.MaxFileSize != nil {
		maxFileSize = *src.MaxFileSize
	}

	enableIPRateLimit := true
	if src.EnableIPRateLimit != nil {
		enableIPRateLimit = *src.EnableIPRateLimit
	}

	redis := resolveRedis(src.Redis, host)

	perChannelMaxNoteCacheCount := 1000
	if src.PerChannelMaxNoteCacheCount != nil {
		perChannelMaxNoteCacheCount = *src.PerChannelMaxNoteCacheCount
	}
	perUserNotificationsMaxCount := 500
	if src.PerUserNotificationsMaxCount != nil {
		perUserNotificationsMaxCount = *src.PerUserNotificationsMaxCount
	}
	deactivateAntennaThreshold := 1000 * 60 * 60 * 24 * 7
	if src.DeactivateAntennaThreshold != nil {
		deactivateAntennaThreshold = *src.DeactivateAntennaThreshold
	}

	internalMediaProxy := fmt.Sprintf("%s://%s/proxy", scheme, host)
	mediaProxy := internalMediaProxy
	externalMediaProxyEnabled := false
	if src.MediaProxy != "" {
		mp := strings.TrimRight(src.MediaProxy, "/")
		if mp != internalMediaProxy {
			externalMediaProxyEnabled = true
		}
		mediaProxy = mp
	}

	mediaProxySecret := deriveMediaProxySecret(src)

	cfg := &Config{
		Version:     MisskeyVersion,
		URL:         parsedURL.Scheme + "://" + parsedURL.Host,
		Port:        src.Port,
		Socket:      src.Socket,
		ChmodSocket: src.ChmodSocket,
		Host:        host,
		Hostname:    hostname,
		Scheme:      scheme,
		WsScheme:    wsScheme,
		WsURL:       fmt.Sprintf("%s://%s", wsScheme, host),
		APIURL:      fmt.Sprintf("%s://%s/api", scheme, host),
		AuthURL:     fmt.Sprintf("%s://%s/auth", scheme, host),
		DriveURL:    fmt.Sprintf("%s://%s/files", scheme, host),

		DisableHSTS:       src.DisableHSTS,
		EnableIPRateLimit: enableIPRateLimit,
		SetupPassword:     src.SetupPassword,

		DB:             src.DB,
		DBReplications: src.DBReplications,

		Redis:             redis,
		RedisForPubsub:    resolveRedisOrDefault(src.RedisForPubsub, redis, host),
		RedisForJobQueue:  resolveRedisOrDefault(src.RedisForJobQueue, redis, host),
		RedisForTimelines: resolveRedisOrDefault(src.RedisForTimelines, redis, host),
		RedisForReactions: resolveRedisOrDefault(src.RedisForReactions, redis, host),

		FulltextSearch: src.FulltextSearch,
		Meilisearch:    src.Meilisearch,

		ID: src.ID,

		Proxy:                  src.Proxy,
		ProxySmtp:              src.ProxySmtp,
		ProxyBypassHosts:       src.ProxyBypassHosts,
		AllowedPrivateNetworks: src.AllowedPrivateNetworks,

		MaxFileSize:           maxFileSize,
		ClusterLimit:          src.ClusterLimit,
		OutgoingAddress:       src.OutgoingAddress,
		OutgoingAddressFamily: src.OutgoingAddressFamily,

		DeliverJobConcurrency:      src.DeliverJobConcurrency,
		InboxJobConcurrency:        src.InboxJobConcurrency,
		RelationshipJobConcurrency: src.RelationshipJobConcurrency,
		DeliverJobPerSec:           src.DeliverJobPerSec,
		InboxJobPerSec:             src.InboxJobPerSec,
		RelationshipJobPerSec:      src.RelationshipJobPerSec,
		DeliverJobMaxAttempts:      src.DeliverJobMaxAttempts,
		InboxJobMaxAttempts:        src.InboxJobMaxAttempts,

		MediaProxy:                   mediaProxy,
		ExternalMediaProxyEnabled:    externalMediaProxyEnabled,
		MediaProxySecret:             mediaProxySecret,
		VideoThumbnailGenerator:      strings.TrimRight(src.VideoThumbnailGenerator, "/"),
		UserAgent:                    fmt.Sprintf("Misskey-Go/%s (%s)", MkGoVersion, src.URL),
		PerChannelMaxNoteCacheCount:  perChannelMaxNoteCacheCount,
		PerUserNotificationsMaxCount: perUserNotificationsMaxCount,
		DeactivateAntennaThreshold:   deactivateAntennaThreshold,
		PidFile:                      src.PidFile,
		Logging:                      src.Logging,

		TrustProxy: resolveTrustProxy(src.TrustProxy),

		TestMode: src.TestMode,
	}

	if cfg.TestMode {
		// TestMode は /api/reset-db のような破壊的エンドポイントを有効化する。
		// 本番で誤って有効化されていないか気付けるよう、起動時に強く警告する。
		slog.Warn("config: TestMode is enabled; destructive test endpoints (e.g. /api/reset-db) are active. DO NOT enable this in production.",
			"url", cfg.URL)
	}

	// HTTP socket と TCP port は両立しない。両方設定されていた場合は
	// socket を優先する旨警告ログを出し、運用者に気付けるようにする。
	if cfg.Socket != "" && cfg.Port != 0 {
		slog.Warn("config: both socket and port are set; socket takes precedence and port will be ignored",
			"socket", cfg.Socket, "port", cfg.Port)
	}

	return cfg, nil
}

func resolveRedis(opts RedisOptions, host string) RedisOptions {
	if opts.Prefix == "" {
		opts.Prefix = host
	}
	return opts
}

// deriveMediaProxySecret returns a secret for HMAC-signed media proxy URLs.
// 設定にsecretがあればそれを使用、なければインスタンスURL固有のキーを自動生成する。
// DefaultTrustProxy is the default set of CIDR ranges for trusted proxies,
// matching the TypeScript Misskey defaults (private IP ranges).
var DefaultTrustProxy = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.1/32",
	"::1/128",
	"fc00::/7",
}

// resolveTrustProxy returns the provided list or the default if empty.
func resolveTrustProxy(provided []string) []string {
	if len(provided) > 0 {
		return provided
	}
	return DefaultTrustProxy
}

func deriveMediaProxySecret(src *Source) []byte {
	if src.MediaProxySecret != "" {
		return []byte(src.MediaProxySecret)
	}
	// URLは公開情報なのでsecretとしては弱い。運用者に設定を促す。
	slog.Warn("config: mediaProxySecret is not set; using a URL-derived fallback. Set mediaProxySecret in config for stronger HMAC security.")
	h := sha256.Sum256([]byte(src.URL + "|mediaproxy"))
	return h[:]
}

func resolveRedisOrDefault(opts *RedisOptions, fallback RedisOptions, host string) RedisOptions {
	if opts == nil {
		return fallback
	}
	return resolveRedis(*opts, host)
}

// DSN returns the PostgreSQL connection string.
//
// Host が "/" で始まる場合は UNIX domain socket 接続とみなす。libpq / pgx の
// 慣例に従い、host にソケットディレクトリのパスを、port に対応する PG ポート番号
// (socket 名 .s.PGSQL.<port> の末尾数字) を渡す。UDS では TLS を張れないので
// sslmode は強制的に disable になる。
func (c *Config) DSN() string {
	if IsUnixSocketPath(c.DB.Host) {
		return fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			c.DB.Host, c.DB.Port, c.DB.User, c.DB.Pass, c.DB.DB,
		)
	}
	sslMode := "disable"
	if v, ok := c.DB.Extra["ssl"]; ok && v == "true" {
		sslMode = "require"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.DB.Host, c.DB.Port, c.DB.User, c.DB.Pass, c.DB.DB, sslMode,
	)
}

// IsUnixSocketPath reports whether the given host string points to a UNIX
// domain socket path. The convention is that any absolute path (starts with
// "/") is treated as a socket. Empty strings and TCP hostnames return false.
//
// This helper is shared between the DB DSN builder, the Redis client factory
// and the HTTP server startup path so that all three sub-systems agree on
// what counts as a UDS address.
func IsUnixSocketPath(host string) bool {
	return strings.HasPrefix(host, "/")
}
