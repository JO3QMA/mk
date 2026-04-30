package server

import (
	"fmt"
	"strings"

	"github.com/shiroha-a/mk/internal/config"
	coresearch "github.com/shiroha-a/mk/internal/core/search"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/repository"
)

// buildSearchProvider selects the search.Provider implementation based on the
// fulltextSearch.provider config key. Provider 選択は起動時に一度だけ走る。
//
//   - meilisearch (with meilisearch host configured) → MeilisearchProvider
//   - sqlPgroonga                                    → SQLLikeProvider with
//     PGroonga `&@~` 演算子 (要 pgroonga 拡張)
//   - sqlLike / unset (with no meilisearch host)     → SQLLikeProvider (ILIKE)
func buildSearchProvider(
	cfg *config.Config,
	noteRepo repository.NoteRepository,
	followingRepo repository.FollowingRepository,
	idGen id.Generator,
) coresearch.Provider {
	provider := ""
	if cfg != nil && cfg.FulltextSearch != nil {
		provider = strings.ToLower(strings.TrimSpace(cfg.FulltextSearch.Provider))
	}
	if provider == "meilisearch" && cfg != nil && cfg.Meilisearch != nil && cfg.Meilisearch.Host != "" {
		host := buildMeilisearchHost(cfg.Meilisearch)
		svc := coresearch.NewMeilisearchService(host, cfg.Meilisearch.APIKey)
		indexName := cfg.Meilisearch.Index
		if indexName == "" {
			indexName = "misskey---notes"
		} else {
			indexName += "---notes"
		}
		client := coresearch.OpenIndex(svc, indexName)
		// インデックス設定の適用は best-effort。失敗してもサーバ起動は続行する。
		mp := coresearch.NewMeilisearchProvider(client, noteRepo, followingRepo, idGen, coresearch.IndexScope(cfg.Meilisearch.Scope))
		_ = mp.ApplyDefaultSettings()
		return mp
	}
	if provider == "sqlpgroonga" {
		return coresearch.NewSQLPgroongaProvider(noteRepo, followingRepo)
	}
	// fulltextSearch.provider が "sqlLike" / 未設定 / 不明な値の場合は
	// SQL ILIKE フォールバック。
	return coresearch.NewSQLLikeProvider(noteRepo, followingRepo)
}

// buildMeilisearchHost composes the http(s)://host:port URL Meilisearch
// expects from Misskey の YAML 設定。host のみで scheme を持たない値も許容する。
func buildMeilisearchHost(opts *config.MeilisearchOptions) string {
	host := opts.Host
	if !strings.Contains(host, "://") {
		scheme := "http"
		if opts.SSL {
			scheme = "https"
		}
		if opts.Port != "" {
			host = fmt.Sprintf("%s://%s:%s", scheme, host, opts.Port)
		} else {
			host = fmt.Sprintf("%s://%s", scheme, host)
		}
	}
	return host
}
