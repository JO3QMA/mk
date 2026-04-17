// Package sentry wires the getsentry/sentry-go client to the Misskey
// configuration and exposes a thin Echo middleware that captures panics and
// handler errors. Lives in its own package so its tests do not influence the
// surrounding subsystem coverage.
package sentry

import (
	"fmt"
	"log/slog"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/config"
)

// flushTimeout is the maximum duration to block draining queued events when
// Flush is called at shutdown.
const flushTimeout = 2 * time.Second

// Init initializes the global sentry-go client when cfg.SentryForBackend is
// non-nil. The returned flush function is always safe to call (it is a no-op
// when Sentry is disabled) and the caller is expected to defer it.
//
// Returning the flush func through the API — rather than using
// sentry.Flush directly at the call site — keeps callers free of a direct
// sentry-go import for the disabled case.
func Init(cfg *config.Config) (func(), error) {
	if cfg == nil || cfg.SentryForBackend == nil {
		return func() {}, nil
	}
	opts := cfg.SentryForBackend.Options
	if opts.DSN == "" {
		// DSN 必須。空のまま init すると sentry-go は環境変数 SENTRY_DSN を
		// 探しに行くので、設定意図が曖昧になる。明示的に弾く。
		return nil, fmt.Errorf("sentry: SentryForBackend.Options.DSN is required when sentryForBackend is set")
	}
	if cfg.SentryForBackend.EnableNodeProfiling {
		// Node 専用機能なので Go では何もしない。設定者に気付かせるため起動時に通知。
		slog.Info("sentry: enableNodeProfiling is a no-op on the Go backend; ignored")
	}
	if err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn:              opts.DSN,
		Environment:      opts.Environment,
		Release:          opts.Release,
		SampleRate:       sampleRateOrDefault(opts.SampleRate),
		TracesSampleRate: opts.TracesSampleRate,
		Debug:            opts.Debug,
		ServerName:       opts.ServerName,
	}); err != nil {
		return nil, fmt.Errorf("sentry init: %w", err)
	}
	slog.Info("sentry: initialized", "environment", opts.Environment, "release", opts.Release)
	return func() { sentrygo.Flush(flushTimeout) }, nil
}

// sampleRateOrDefault treats a zero rate as "use sentry-go default" because
// 0.0 means "drop everything" which is rarely the user's intent. Misskey's
// upstream defaults to 1.0 (capture all errors).
func sampleRateOrDefault(r float64) float64 {
	if r <= 0 {
		return 1.0
	}
	return r
}

// Middleware returns an Echo middleware that forwards panics and handler
// errors to the global sentry-go client. When Sentry is disabled the
// middleware short-circuits to a pass-through to keep the request hot path
// allocation-free.
func Middleware(cfg *config.Config) echo.MiddlewareFunc {
	if cfg == nil || cfg.SentryForBackend == nil {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			// 各リクエスト毎に hub を分離 (タグ・ユーザー情報がリクエスト境界で混ざらない)
			hub := sentrygo.CurrentHub().Clone()
			hub.Scope().SetRequest(c.Request())
			defer func() {
				if r := recover(); r != nil {
					hub.RecoverWithContext(c.Request().Context(), r)
					// echo の Recover middleware に巻き戻す
					panic(r)
				}
			}()
			err = next(c)
			if err != nil {
				hub.CaptureException(err)
			}
			return err
		}
	}
}
