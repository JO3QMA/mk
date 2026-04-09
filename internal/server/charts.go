package server

import (
	apicharts "github.com/shiroha-a/mk/internal/api/charts"
	"github.com/shiroha-a/mk/internal/core/chart"
	corechartcharts "github.com/shiroha-a/mk/internal/core/chart/charts"
	"gorm.io/gorm"
)

// buildChartBundle constructs every chart engine the server needs and
// returns them packaged for the API handler. Each engine gets its own
// gormRepository instance bound to its specific schema; lock and clock
// are shared singletons. The function is package-level so router.go
// stays focused on routing.
func buildChartBundle(db *gorm.DB) apicharts.Charts {
	locker := chart.NewMemoryLocker()
	mk := func(schema chart.Schema) *chart.Chart {
		c, err := chart.New(chart.Config{
			Schema: schema,
			Repo:   chart.NewRepository(db, schema),
			Lock:   locker,
		})
		if err != nil {
			// Schema 不正は起動時に検出したいので panic で弾く。
			// 各 SchemaXxx() は定数を返すので実運用ではここに来ない。
			panic(err)
		}
		return c
	}
	return apicharts.Charts{
		Notes:            mk(corechartcharts.SchemaNotes()),
		Users:            mk(corechartcharts.SchemaUsers()),
		Drive:            mk(corechartcharts.SchemaDrive()),
		Federation:       mk(corechartcharts.SchemaFederation()),
		Instance:         mk(corechartcharts.SchemaInstance()),
		ApRequest:        mk(corechartcharts.SchemaApRequest()),
		ActiveUsers:      mk(corechartcharts.SchemaActiveUsers()),
		PerUserNotes:     mk(corechartcharts.SchemaPerUserNotes()),
		PerUserDrive:     mk(corechartcharts.SchemaPerUserDrive()),
		PerUserFollowing: mk(corechartcharts.SchemaPerUserFollowing()),
		PerUserPv:        mk(corechartcharts.SchemaPerUserPv()),
		PerUserReaction:  mk(corechartcharts.SchemaPerUserReaction()),
	}
}
