package server

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	apiadmin "github.com/shiroha-a/mk/internal/api/admin"
	apiannouncements "github.com/shiroha-a/mk/internal/api/announcements"
	"github.com/shiroha-a/mk/internal/api/antennas"
	"github.com/shiroha-a/mk/internal/api/ap"
	"github.com/shiroha-a/mk/internal/api/blocking"
	apichannels "github.com/shiroha-a/mk/internal/api/channels"
	apicharts "github.com/shiroha-a/mk/internal/api/charts"
	"github.com/shiroha-a/mk/internal/api/clips"
	"github.com/shiroha-a/mk/internal/api/drive"
	apiemojis "github.com/shiroha-a/mk/internal/api/emojis"
	apifederation "github.com/shiroha-a/mk/internal/api/federation"
	apiflash "github.com/shiroha-a/mk/internal/api/flash"
	"github.com/shiroha-a/mk/internal/api/following"
	apigallery "github.com/shiroha-a/mk/internal/api/gallery"
	apihashtags "github.com/shiroha-a/mk/internal/api/hashtags"
	"github.com/shiroha-a/mk/internal/api/i"
	"github.com/shiroha-a/mk/internal/api/inbox"
	"github.com/shiroha-a/mk/internal/api/meta"
	"github.com/shiroha-a/mk/internal/api/mute"
	"github.com/shiroha-a/mk/internal/api/nodeinfo"
	"github.com/shiroha-a/mk/internal/api/notes"
	"github.com/shiroha-a/mk/internal/api/notifications"
	"github.com/shiroha-a/mk/internal/api/pages"
	"github.com/shiroha-a/mk/internal/api/renotemute"
	apiroles "github.com/shiroha-a/mk/internal/api/roles"
	apisignin "github.com/shiroha-a/mk/internal/api/signin"
	"github.com/shiroha-a/mk/internal/api/streaming"
	apiuserlists "github.com/shiroha-a/mk/internal/api/userlists"
	"github.com/shiroha-a/mk/internal/api/users"
	"github.com/shiroha-a/mk/internal/api/wellknown"
	coreantenna "github.com/shiroha-a/mk/internal/core/antenna"
	coreblocking "github.com/shiroha-a/mk/internal/core/blocking"
	corechannel "github.com/shiroha-a/mk/internal/core/channel"
	"github.com/shiroha-a/mk/internal/core/chart"
	"github.com/shiroha-a/mk/internal/core/chart/charthook"
	coreclip "github.com/shiroha-a/mk/internal/core/clip"
	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/core/event"
	corefederation "github.com/shiroha-a/mk/internal/core/federation"
	coreflash "github.com/shiroha-a/mk/internal/core/flash"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	coreinstance "github.com/shiroha-a/mk/internal/core/instance"
	coremuting "github.com/shiroha-a/mk/internal/core/muting"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	corenotification "github.com/shiroha-a/mk/internal/core/notification"
	corepage "github.com/shiroha-a/mk/internal/core/page"
	corepoll "github.com/shiroha-a/mk/internal/core/poll"
	corereaction "github.com/shiroha-a/mk/internal/core/reaction"
	corerole "github.com/shiroha-a/mk/internal/core/role"
	coresearch "github.com/shiroha-a/mk/internal/core/search"
	coresignup "github.com/shiroha-a/mk/internal/core/signup"
	coretimeline "github.com/shiroha-a/mk/internal/core/timeline"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/entity"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/model"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/stream"
	"github.com/shiroha-a/mk/internal/stream/channels"
	"log/slog"
)

func (s *Server) setupRoutes() {
	idGen, err := id.NewGenerator(s.config.ID)
	if err != nil {
		idGen, _ = id.NewGenerator("aidx")
	}

	// Repositories
	userRepo := repository.NewUserRepository(s.db)
	noteRepo := repository.NewNoteRepository(s.db)
	metaRepo := repository.NewMetaRepository(s.db)
	pollRepo := repository.NewPollRepository(s.db)
	followingRepo := repository.NewFollowingRepository(s.db)
	followRequestRepo := repository.NewFollowRequestRepository(s.db)
	piningRepo := repository.NewUserNotePiningRepository(s.db)
	reactionRepo := repository.NewNoteReactionRepository(s.db)
	emojiRepo := repository.NewEmojiRepository(s.db)
	blockingRepo := repository.NewBlockingRepository(s.db)
	mutingRepo := repository.NewMutingRepository(s.db)
	renoteMutingRepo := repository.NewRenoteMutingRepository(s.db)
	pollVoteRepo := repository.NewPollVoteRepository(s.db)
	driveFileRepo := repository.NewDriveFileRepository(s.db)
	driveFolderRepo := repository.NewDriveFolderRepository(s.db)
	keypairRepo := repository.NewUserKeypairRepository(s.db)
	instanceRepo := repository.NewInstanceRepository(s.db)
	channelRepo := repository.NewChannelRepository(s.db)
	channelFollowingRepo := repository.NewChannelFollowingRepository(s.db)
	antennaRepo := repository.NewAntennaRepository(s.db)
	clipRepo := repository.NewClipRepository(s.db)
	clipNoteRepo := repository.NewClipNoteRepository(s.db)
	pageRepo := repository.NewPageRepository(s.db)
	pageLikeRepo := repository.NewPageLikeRepository(s.db)
	flashRepo := repository.NewFlashRepository(s.db)
	flashLikeRepo := repository.NewFlashLikeRepository(s.db)
	roleRepo := repository.NewRoleRepository(s.db)
	roleAssignmentRepo := repository.NewRoleAssignmentRepository(s.db)

	// Core services
	roleService := corerole.NewService(roleRepo, roleAssignmentRepo, metaRepo, idGen)
	signupService := coresignup.NewService(userRepo, metaRepo, idGen)
	noteCreateService := corenote.NewCreateService(noteRepo, pollRepo, idGen, followingRepo)
	noteDeleteService := corenote.NewDeleteService(noteRepo)
	noteQueryService := corenote.NewQueryService(noteRepo, followingRepo)
	userService := coreuser.NewService(userRepo, noteRepo, piningRepo, idGen)
	followingService := corefollowing.NewService(userRepo, followingRepo, followRequestRepo, idGen)

	// Timeline services (Redis-backed fanout)
	fanoutTimelineService := coretimeline.NewFanoutTimelineService(s.redis.Timelines, idGen)
	timelineService := coretimeline.NewService(fanoutTimelineService, noteRepo, followingRepo)
	timelineFanoutHook := coretimeline.NewFanoutHook(fanoutTimelineService, followingRepo)
	noteCreateService.SetFanoutHook(timelineFanoutHook)

	// Channels (Phase 4.2)
	channelService := corechannel.NewService(channelRepo, channelFollowingRepo, noteRepo, idGen)
	noteCreateService.SetChannelHook(corechannel.NewNoteCreateHook(channelService))

	// Antennas (Phase 4.3)
	antennaService := coreantenna.NewService(antennaRepo, userRepo, s.redis.Default, idGen)
	noteCreateService.SetAntennaHook(coreantenna.NewNoteCreateHook(antennaService))

	// Clips (Phase 4.4)
	clipService := coreclip.NewService(clipRepo, clipNoteRepo, noteRepo, idGen)

	// Pages / Flash (Phase 4.5)
	pageService := corepage.NewService(pageRepo, pageLikeRepo, idGen)
	flashService := coreflash.NewService(flashRepo, flashLikeRepo, idGen)

	// Reactions
	reactionService := corereaction.NewService(noteRepo, reactionRepo, emojiRepo, followingRepo, idGen)

	// Notifications (Redis Streams)
	notificationService := corenotification.NewService(s.redis.Default, idGen)
	notificationHook := corenotification.NewHook(notificationService, userRepo)
	noteCreateService.SetNotificationHook(notificationHook)
	followingService.SetNotificationHook(notificationHook)
	reactionService.SetNotificationHook(notificationHook)

	// Blocking & Muting
	blockingService := coreblocking.NewService(userRepo, blockingRepo, followingRepo, idGen)
	mutingService := coremuting.NewService(userRepo, mutingRepo, idGen)
	renoteMutingService := coremuting.NewRenoteService(userRepo, renoteMutingRepo, idGen)
	followingService.SetBlockingChecker(blockingService)
	reactionService.SetBlockingChecker(blockingService)
	notificationHook.SetMuteChecker(mutingService)

	// Polls
	pollService := corepoll.NewService(noteRepo, pollRepo, pollVoteRepo, followingRepo, idGen)
	pollService.SetNotificationHook(notificationHook)

	// Drive storage (Phase 4.9: Meta に基づいて Local / S3 を切り替え)
	var driveStorage coredrive.Storage
	if serverMeta, err := metaRepo.Fetch(); err == nil {
		driveStorage = coredrive.NewStorageFromMeta(serverMeta, "./drive-files", s.config.DriveURL)
	} else {
		driveStorage = coredrive.NewLocalStorage("./drive-files", s.config.DriveURL)
	}
	driveService := coredrive.NewService(driveFileRepo, driveFolderRepo, driveStorage, idGen)

	// Image processing (Phase 4.8)
	imgProcessor := coredrive.NewDefaultImageProcessor()
	driveService.SetImageProcessor(imgProcessor)
	driveService.SetVideoProcessor(coredrive.NewFFmpegVideoProcessor(imgProcessor, nil))

	// Search (Phase 4.6)
	// 設定に従って provider を選択する。Meilisearch が設定されていれば
	// それを使い、そうでなければ SQL ILIKE フォールバック。
	searchService := coresearch.NewService(buildSearchProvider(s.config, noteRepo, followingRepo, idGen))
	noteCreateService.SetIndexHook(coresearch.NewNoteIndexHook(searchService))
	noteDeleteService.SetIndexHook(coresearch.NewNoteIndexHook(searchService))

	// ActivityPub
	apURLs := activitypub.NewURLBuilder(s.config.URL)
	apRenderer := activitypub.NewRenderer(apURLs)
	apClient := activitypub.NewClient(nil, "misskey-go/"+s.config.Version)
	apFetcher := corefederation.NewAPFetcher(apClient)
	federationResolver := corefederation.NewResolver(userRepo, noteRepo, apURLs, apFetcher, idGen)
	federationProcessor := corefederation.NewProcessor(federationResolver, followingService, reactionService, noteDeleteService, userRepo, noteRepo)

	// Instance management (Phase 3 Step H)
	instanceService := coreinstance.NewService(instanceRepo, metaRepo, idGen)
	federationResolver.SetInstanceTracker(instanceService)
	// 新規 instance row 発見時に nodeinfo を取得して metadata を更新する
	instanceService.SetMetadataFetcher(coreinstance.NewFetchMetadataService(instanceRepo, apFetcher))

	// AP delivery: DeliverService + フック登録 + asynq processor 登録
	deliverService := corefederation.NewDeliverService(s.queueClient, userRepo, followingRepo, keypairRepo, apURLs)
	deliverService.SetHostBlockChecker(instanceService)
	noteCreateService.SetFederationHook(corefederation.NewNoteDeliveryHook(deliverService, apRenderer, apURLs, idGen, userRepo, noteRepo))
	followingService.SetFederationHook(corefederation.NewFollowingDeliveryHook(deliverService, apRenderer, apURLs))
	reactionService.SetFederationHook(corefederation.NewReactionDeliveryHook(deliverService, apRenderer, apURLs, idGen, userRepo))
	noteDeleteService.SetFederationHook(corefederation.NewNoteDeleteDeliveryHook(deliverService, apRenderer, apURLs))
	deliverProcessor := processors.NewDeliverProcessor(apClient)
	// 配信結果に応じて instance.isNotResponding を更新する
	deliverProcessor.SetResponseHook(instanceService)
	s.queueServer.Handle(queue.TaskTypeDeliver, deliverProcessor.Handle)

	// Charts (Phase 4 Step Q / 4.7) — 12 chart engines + management
	// service + hook adapter. The hooks must be wired before any
	// service that consumes them is invoked, so we construct the
	// bundle here, before the handler-binding section below.
	chartCharts := buildChartBundle(s.db)
	chartMgmt := chart.NewManagementService([]*chart.Chart{
		chartCharts.Notes,
		chartCharts.Users,
		chartCharts.Drive,
		chartCharts.Federation,
		chartCharts.Instance,
		chartCharts.ApRequest,
		chartCharts.ActiveUsers,
		chartCharts.PerUserNotes,
		chartCharts.PerUserDrive,
		chartCharts.PerUserFollowing,
		chartCharts.PerUserPv,
		chartCharts.PerUserReaction,
	}, 0)
	s.setChartManagement(chartMgmt)
	chartHooks := charthook.New(charthook.Config{
		Notes:            chartCharts.Notes,
		Users:            chartCharts.Users,
		Drive:            chartCharts.Drive,
		Federation:       chartCharts.Federation,
		Instance:         chartCharts.Instance,
		ApRequest:        chartCharts.ApRequest,
		ActiveUsers:      chartCharts.ActiveUsers,
		PerUserNotes:     chartCharts.PerUserNotes,
		PerUserDrive:     chartCharts.PerUserDrive,
		PerUserFollowing: chartCharts.PerUserFollowing,
		PerUserPv:        chartCharts.PerUserPv,
		PerUserReaction:  chartCharts.PerUserReaction,
		IDGen:            idGen,
	})
	// 各サービスへ chart hook を注入する。Set* は nil 安全なので順序は不問。
	noteCreateService.SetChartHook(chartHooks)
	noteDeleteService.SetChartHook(chartHooks)
	followingService.SetChartHook(chartHooks)
	reactionService.SetChartHook(chartHooks)
	driveService.SetChartHook(chartHooks)
	federationResolver.SetChartHook(chartHooks)
	deliverProcessor.SetChartHook(chartHooks)

	// Health check
	s.echo.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api := s.echo.Group("/api")

	// Meta endpoint (public)
	metaHandler := meta.NewHandler(s.config, metaRepo)
	api.POST("/meta", metaHandler.Meta)
	api.POST("/ping", metaHandler.Ping)

	// Stats endpoint (public) — チャートの集計済み値から取得
	notesChart := chartCharts.Notes
	usersChart := chartCharts.Users
	api.POST("/stats", func(c echo.Context) error {
		ctx := c.Request().Context()
		var notesCount, originalNotesCount, usersCount, originalUsersCount int64

		if res, err := notesChart.GetChart(ctx, chart.SpanHour, 1, nil, ""); err == nil {
			if v, ok := res["local.total"]; ok && len(v) > 0 {
				originalNotesCount = v[0]
			}
			if v, ok := res["remote.total"]; ok && len(v) > 0 {
				notesCount = originalNotesCount + v[0]
			}
		}
		if res, err := usersChart.GetChart(ctx, chart.SpanHour, 1, nil, ""); err == nil {
			if v, ok := res["local.total"]; ok && len(v) > 0 {
				originalUsersCount = v[0]
			}
			if v, ok := res["remote.total"]; ok && len(v) > 0 {
				usersCount = originalUsersCount + v[0]
			}
		}

		var instancesCount int64
		s.db.Model(&model.Instance{}).Count(&instancesCount)

		return c.JSON(http.StatusOK, map[string]any{
			"notesCount":         notesCount,
			"originalNotesCount": originalNotesCount,
			"usersCount":         usersCount,
			"originalUsersCount": originalUsersCount,
			"instances":          instancesCount,
			"driveUsageLocal":    0,
			"driveUsageRemote":   0,
			"reactionsCount":     0,
		})
	})

	// Users endpoint (public) — ユーザー一覧
	api.POST("/users", func(c echo.Context) error {
		var req struct {
			Limit  int    `json:"limit"`
			Offset int    `json:"offset"`
			Sort   string `json:"sort"`
			State  string `json:"state"`
			Origin string `json:"origin"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusOK, []any{})
		}
		if req.Limit <= 0 {
			req.Limit = 10
		}
		if req.Origin == "" {
			req.Origin = "local"
		}
		users, err := userRepo.ListUsers(model.UserListFilter{
			State: req.State, Origin: req.Origin, Sort: req.Sort,
			Limit: req.Limit, Offset: req.Offset,
		})
		if err != nil {
			return c.JSON(http.StatusOK, []any{})
		}
		result := make([]entity.UserDetailed, 0, len(users))
		for _, u := range users {
			profile, _ := userRepo.FindProfileByUserID(u.ID)
			result = append(result, entity.PackUserDetailed(u, profile))
		}
		return c.JSON(http.StatusOK, result)
	})

	// Pinned users (public)
	api.POST("/pinned-users", func(c echo.Context) error {
		m, err := metaRepo.Fetch()
		if err != nil || len(m.PinnedUsers) == 0 {
			return c.JSON(http.StatusOK, []any{})
		}
		var result []entity.UserLite
		for _, username := range m.PinnedUsers {
			if u, err := userRepo.FindByUsernameLower(username, nil); err == nil {
				result = append(result, entity.PackUserLite(u))
			}
		}
		if result == nil {
			result = []entity.UserLite{}
		}
		return c.JSON(http.StatusOK, result)
	})

	// Signin (Phase 6)
	signinHandler := apisignin.NewHandler(userRepo)
	api.POST("/signin", signinHandler.Signin)
	api.POST("/signin-flow", signinHandler.SigninFlow)

	// Emojis endpoint (public, Phase 4.5i)
	emojisHandler := apiemojis.NewHandler(emojiRepo)
	api.POST("/emojis", emojisHandler.Emojis)

	// Notes endpoints
	notesHandler := notes.NewHandler(noteRepo, noteCreateService, noteDeleteService, noteQueryService, timelineService, reactionService, pollService, searchService, idGen)
	api.POST("/notes/create", notesHandler.Create, middleware.RequireAuth())
	api.POST("/notes/show", notesHandler.Show)
	api.POST("/notes/delete", notesHandler.Delete, middleware.RequireAuth())
	api.POST("/notes/renotes", notesHandler.Renotes)
	api.POST("/notes/replies", notesHandler.Replies)
	api.POST("/notes/children", notesHandler.Children)
	api.POST("/notes/conversation", notesHandler.Conversation)
	api.POST("/notes/search", notesHandler.Search)
	api.POST("/notes/state", notesHandler.State, middleware.RequireAuth())
	api.POST("/notes/timeline", notesHandler.Timeline, middleware.RequireAuth())
	api.POST("/notes/local-timeline", notesHandler.LocalTimeline)
	api.POST("/notes/global-timeline", notesHandler.GlobalTimeline)
	api.POST("/notes/hybrid-timeline", notesHandler.HybridTimeline, middleware.RequireAuth())
	api.POST("/notes/reactions", notesHandler.Reactions)
	api.POST("/notes/reactions/create", notesHandler.ReactionsCreate, middleware.RequireAuth())
	api.POST("/notes/reactions/delete", notesHandler.ReactionsDelete, middleware.RequireAuth())
	api.POST("/notes/polls/vote", notesHandler.PollsVote, middleware.RequireAuth())
	// Notes extra endpoints (Phase 6)
	noteFavoriteRepo := repository.NewNoteFavoriteRepository(s.db)
	notesHandler.SetFavoriteRepo(noteFavoriteRepo)
	api.POST("/notes/favorites/create", notesHandler.FavoritesCreate, middleware.RequireAuth())
	api.POST("/notes/favorites/delete", notesHandler.FavoritesDelete, middleware.RequireAuth())
	api.POST("/notes/featured", notesHandler.Featured)
	api.GET("/notes/featured", notesHandler.Featured)
	api.POST("/notes/unrenote", notesHandler.Unrenote, middleware.RequireAuth())
	api.POST("/notes/mentions", notesHandler.Mentions, middleware.RequireAuth())
	api.POST("/notes/user-list-timeline", notesHandler.UserListTimeline, middleware.RequireAuth())

	// Users endpoints
	usersHandler := users.NewHandler(userService, followingService, noteRepo, idGen)
	usersHandler.SetChartHook(chartHooks)
	api.POST("/users/show", usersHandler.Show)
	api.POST("/users/search", usersHandler.Search)
	api.POST("/users/notes", usersHandler.Notes)
	api.POST("/users/followers", usersHandler.Followers)
	api.POST("/users/following", usersHandler.Following)
	api.POST("/users/relation", usersHandler.Relation, middleware.RequireAuth())
	api.POST("/users/report-abuse", usersHandler.ReportAbuse, middleware.RequireAuth())
	api.POST("/users/reactions", usersHandler.Reactions)
	api.POST("/users/featured-notes", usersHandler.FeaturedNotes)
	api.POST("/users/search-by-username-and-host", usersHandler.SearchByUsernameAndHost)
	api.POST("/users/update-memo", usersHandler.UpdateMemo, middleware.RequireAuth())
	usersHandler.SetAbuseRepo(repository.NewAbuseReportRepository(s.db))

	// Account endpoints
	registryRepo := repository.NewRegistryRepository(s.db)
	iHandler := i.NewHandler(userService, idGen)
	iHandler.SetRoleProvider(roleService)
	iHandler.SetRegistryRepo(registryRepo)
	api.POST("/i", iHandler.Me, middleware.RequireAuth())
	api.POST("/i/update", iHandler.Update, middleware.RequireAuth())
	api.POST("/i/pin", iHandler.Pin, middleware.RequireAuth())
	api.POST("/i/unpin", iHandler.Unpin, middleware.RequireAuth())
	api.POST("/i/registry/get", iHandler.RegistryGet, middleware.RequireAuth())
	api.POST("/i/registry/set", iHandler.RegistrySet, middleware.RequireAuth())
	api.POST("/i/registry/get-all", iHandler.RegistryGetAll, middleware.RequireAuth())
	api.POST("/i/registry/keys-with-type", iHandler.RegistryKeysWithType, middleware.RequireAuth())
	api.POST("/i/registry/remove", iHandler.RegistryRemove, middleware.RequireAuth())
	api.POST("/i/change-password", iHandler.ChangePassword, middleware.RequireAuth())
	api.POST("/i/delete-account", iHandler.DeleteAccount, middleware.RequireAuth())
	api.POST("/i/favorites", iHandler.Favorites, middleware.RequireAuth())
	api.POST("/i/notifications-grouped", iHandler.NotificationsGrouped, middleware.RequireAuth())
	api.POST("/i/regenerate-token", iHandler.RegenerateToken, middleware.RequireAuth())
	iHandler.SetFavoriteRepo(noteFavoriteRepo)

	// Notifications endpoints
	notificationsHandler := notifications.NewHandler(notificationService, idGen)
	api.POST("/i/notifications", notificationsHandler.Show, middleware.RequireAuth())
	api.POST("/notifications/mark-all-as-read", notificationsHandler.MarkAllAsRead, middleware.RequireAuth())
	api.POST("/notifications/create", notificationsHandler.Create, middleware.RequireAuth())
	api.POST("/notifications/flush", notificationsHandler.Flush, middleware.RequireAuth())
	api.POST("/notifications/test-notification", notificationsHandler.TestNotification, middleware.RequireAuth())

	// Phase 7.1a: フロントエンドが呼ぶ i/* スタブ
	api.POST("/i/claim-achievement", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, middleware.RequireAuth())
	api.POST("/i/apps", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAuth())
	api.POST("/i/authorized-apps", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAuth())
	api.POST("/i/signin-history", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAuth())
	api.POST("/i/revoke-token", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, middleware.RequireAuth())

	// Hashtags endpoints (Phase 6)
	hashtagsHandler := apihashtags.NewHandler(s.db)
	api.POST("/hashtags/list", hashtagsHandler.List)
	api.POST("/hashtags/search", hashtagsHandler.Search)
	api.POST("/hashtags/show", hashtagsHandler.Show)
	api.POST("/hashtags/trend", hashtagsHandler.Trend)
	api.POST("/hashtags/users", hashtagsHandler.Users)

	// Gallery endpoints (Phase 6)
	galleryHandler := apigallery.NewHandler(s.db, idGen)
	api.POST("/gallery/featured", galleryHandler.Featured)
	api.POST("/gallery/popular", galleryHandler.Popular)
	api.POST("/gallery/posts", galleryHandler.Posts)
	api.POST("/gallery/posts/create", galleryHandler.PostsCreate, middleware.RequireAuth())
	api.POST("/gallery/posts/show", galleryHandler.PostsShow)
	api.POST("/gallery/posts/delete", galleryHandler.PostsDelete, middleware.RequireAuth())
	api.POST("/gallery/posts/update", galleryHandler.PostsUpdate, middleware.RequireAuth())
	api.POST("/gallery/posts/like", galleryHandler.PostsLike, middleware.RequireAuth())
	api.POST("/gallery/posts/unlike", galleryHandler.PostsUnlike, middleware.RequireAuth())

	// Blocking endpoints
	blockingHandler := blocking.NewHandler(blockingService)
	api.POST("/blocking/create", blockingHandler.Create, middleware.RequireAuth())
	api.POST("/blocking/delete", blockingHandler.Delete, middleware.RequireAuth())
	api.POST("/blocking/list", blockingHandler.List, middleware.RequireAuth())

	// Mute endpoints
	muteHandler := mute.NewHandler(mutingService)
	api.POST("/mute/create", muteHandler.Create, middleware.RequireAuth())
	api.POST("/mute/delete", muteHandler.Delete, middleware.RequireAuth())
	api.POST("/mute/list", muteHandler.List, middleware.RequireAuth())

	// Renote mute endpoints
	renoteMuteHandler := renotemute.NewHandler(renoteMutingService)
	api.POST("/renote-mute/create", renoteMuteHandler.Create, middleware.RequireAuth())
	api.POST("/renote-mute/delete", renoteMuteHandler.Delete, middleware.RequireAuth())
	api.POST("/renote-mute/list", renoteMuteHandler.List, middleware.RequireAuth())

	// Drive endpoints
	driveHandler := drive.NewHandler(driveService, idGen)
	api.POST("/drive/files/create", driveHandler.FilesCreate, middleware.RequireAuth())
	api.POST("/drive/files/show", driveHandler.FilesShow, middleware.RequireAuth())
	api.POST("/drive/files/update", driveHandler.FilesUpdate, middleware.RequireAuth())
	api.POST("/drive/files/delete", driveHandler.FilesDelete, middleware.RequireAuth())
	api.POST("/drive/files/find-by-hash", driveHandler.FilesFindByHash, middleware.RequireAuth())
	api.POST("/drive/folders/create", driveHandler.FoldersCreate, middleware.RequireAuth())
	api.POST("/drive/folders/show", driveHandler.FoldersShow, middleware.RequireAuth())
	api.POST("/drive/folders/update", driveHandler.FoldersUpdate, middleware.RequireAuth())
	api.POST("/drive/folders/delete", driveHandler.FoldersDelete, middleware.RequireAuth())

	// Static file serving for LocalStorage
	// MIME type はファイル内容の先頭から自動判定する。
	s.echo.GET("/files/:accessKey", func(c echo.Context) error {
		key := c.Param("accessKey")
		body, err := driveStorage.Get(key)
		if err != nil {
			return c.NoContent(http.StatusNotFound)
		}
		defer body.Close()

		// 先頭512バイトを読んでMIME typeを判定
		buf := make([]byte, 512)
		n, _ := body.Read(buf)
		contentType := http.DetectContentType(buf[:n])
		// 読んだ分 + 残りを連結して配信
		mr := io.MultiReader(bytes.NewReader(buf[:n]), body)
		return c.Stream(http.StatusOK, contentType, mr)
	})

	// ActivityPub resource endpoints
	apHandler := ap.NewHandler(apRenderer, userService, noteQueryService, keypairRepo, idGen)
	s.echo.GET("/users/:id", apHandler.User)
	s.echo.GET("/notes/:id", apHandler.Note)

	// Discovery endpoints
	wellknownHandler := wellknown.NewHandler(apURLs, userService, s.config.Host)
	s.echo.GET("/.well-known/webfinger", wellknownHandler.Webfinger)
	s.echo.GET("/.well-known/host-meta", wellknownHandler.HostMeta)
	s.echo.GET("/.well-known/nodeinfo", wellknownHandler.NodeInfoDiscovery)

	nodeinfoHandler := nodeinfo.NewHandler(s.config)
	s.echo.GET("/nodeinfo/2.1", nodeinfoHandler.Version2_1)

	// Inbox endpoints
	inboxHandler := inbox.NewHandler(federationResolver, federationProcessor)
	inboxHandler.SetHostBlockChecker(instanceService)
	inboxHandler.SetInstanceTracker(instanceService)
	inboxHandler.SetChartHook(chartHooks)
	s.echo.POST("/inbox", inboxHandler.Inbox)
	s.echo.POST("/users/:id/inbox", inboxHandler.Inbox)

	// Federation endpoints
	federationHandler := apifederation.NewHandler(instanceService)
	api.POST("/federation/instances", federationHandler.Instances)
	api.GET("/federation/instances", federationHandler.Instances)
	api.POST("/federation/show-instance", federationHandler.ShowInstance)

	// Channels endpoints (Phase 4.2)
	channelsHandler := apichannels.NewHandler(channelService, idGen)
	api.POST("/channels/create", channelsHandler.Create, middleware.RequireAuth())
	api.POST("/channels/show", channelsHandler.Show)
	api.POST("/channels/update", channelsHandler.Update, middleware.RequireAuth())
	api.POST("/channels/follow", channelsHandler.Follow, middleware.RequireAuth())
	api.POST("/channels/unfollow", channelsHandler.Unfollow, middleware.RequireAuth())
	api.POST("/channels/followed", channelsHandler.Followed, middleware.RequireAuth())
	api.POST("/channels/owned", channelsHandler.Owned, middleware.RequireAuth())
	api.POST("/channels/featured", channelsHandler.Featured)
	api.POST("/channels/search", channelsHandler.Search)
	api.POST("/channels/timeline", channelsHandler.Timeline)

	// Antennas endpoints (Phase 4.3)
	antennasHandler := antennas.NewHandler(antennaService, noteRepo, idGen)
	api.POST("/antennas/create", antennasHandler.Create, middleware.RequireAuth())
	api.POST("/antennas/show", antennasHandler.Show, middleware.RequireAuth())
	api.POST("/antennas/update", antennasHandler.Update, middleware.RequireAuth())
	api.POST("/antennas/delete", antennasHandler.Delete, middleware.RequireAuth())
	api.POST("/antennas/list", antennasHandler.List, middleware.RequireAuth())
	api.POST("/antennas/notes", antennasHandler.Notes, middleware.RequireAuth())

	// Clips endpoints (Phase 4.4)
	clipsHandler := clips.NewHandler(clipService, idGen)
	api.POST("/clips/create", clipsHandler.Create, middleware.RequireAuth())
	api.POST("/clips/show", clipsHandler.Show)
	api.POST("/clips/update", clipsHandler.Update, middleware.RequireAuth())
	api.POST("/clips/delete", clipsHandler.Delete, middleware.RequireAuth())
	api.POST("/clips/list", clipsHandler.List, middleware.RequireAuth())
	api.POST("/clips/add-note", clipsHandler.AddNote, middleware.RequireAuth())
	api.POST("/clips/remove-note", clipsHandler.RemoveNote, middleware.RequireAuth())
	api.POST("/clips/notes", clipsHandler.Notes)

	// Pages endpoints (Phase 4.5)
	pagesHandler := pages.NewHandler(pageService)
	api.POST("/pages/create", pagesHandler.Create, middleware.RequireAuth())
	api.POST("/pages/show", pagesHandler.Show)
	api.POST("/pages/update", pagesHandler.Update, middleware.RequireAuth())
	api.POST("/pages/delete", pagesHandler.Delete, middleware.RequireAuth())
	api.POST("/pages/featured", pagesHandler.Featured)
	api.POST("/pages/like", pagesHandler.Like, middleware.RequireAuth())
	api.POST("/pages/unlike", pagesHandler.Unlike, middleware.RequireAuth())
	api.POST("/i/pages", pagesHandler.My, middleware.RequireAuth())

	// Flash endpoints (Phase 4.5)
	flashHandler := apiflash.NewHandler(flashService)
	api.POST("/flash/create", flashHandler.Create, middleware.RequireAuth())
	api.POST("/flash/show", flashHandler.Show)
	api.POST("/flash/update", flashHandler.Update, middleware.RequireAuth())
	api.POST("/flash/delete", flashHandler.Delete, middleware.RequireAuth())
	api.POST("/flash/featured", flashHandler.Featured)
	api.POST("/flash/search", flashHandler.Search)
	api.POST("/flash/like", flashHandler.Like, middleware.RequireAuth())
	api.POST("/flash/unlike", flashHandler.Unlike, middleware.RequireAuth())
	api.POST("/i/flashs", flashHandler.My, middleware.RequireAuth())
	api.POST("/i/flashs/likes", flashHandler.MyLikes, middleware.RequireAuth())

	// Streaming (Phase 4.1 Step K)
	// 1. Redis pubsub bus (核となる publish/subscribe チャンネル)
	streamPubSub := event.NewPubSubService(s.redis.Pubsub, "stream:")
	streamBus := stream.NewEventPubSubBus(streamPubSub)

	// 2. Channel registry: Misskey 互換のチャンネル名で各 factory を登録する
	streamRegistry := stream.NewRegistry()
	streamRegistry.Register("homeTimeline", channels.NewHomeTimeline)
	streamRegistry.Register("localTimeline", channels.NewLocalTimeline)
	streamRegistry.Register("globalTimeline", channels.NewGlobalTimeline)
	streamRegistry.Register("hybridTimeline", channels.NewHybridTimeline)
	streamRegistry.Register("notifications", channels.NewNotifications)
	streamRegistry.Register("main", channels.NewMain)
	streamRegistry.Register("drive", channels.NewDrive)

	// 3. Connection manager + 各 publisher の生成
	streamManager := stream.NewManager(streamRegistry, streamBus)
	notePublisher := stream.NewNotePublisher(streamPubSub, idGen)
	notificationPublisher := stream.NewNotificationPublisher(streamPubSub)
	drivePublisher := stream.NewDrivePublisher(streamPubSub)

	// 4. 既存サービスへ publisher を注入する。これらはいずれも nil 安全な
	//    setter で、未設定なら何もしない (テスト互換)。
	timelineFanoutHook.SetStreamingPublisher(notePublisher)
	notificationService.SetStreamingPublisher(notificationPublisher)
	driveService.SetStreamingPublisher(drivePublisher)

	// 5. /streaming エンドポイント配線
	streamingHandler := streaming.NewHandler(streamManager)
	s.echo.GET("/streaming", streamingHandler.Stream)

	// Charts API endpoints (engines + hooks already wired earlier).
	chartsHandler := apicharts.NewHandler(chartCharts, nil)
	api.POST("/charts/notes", chartsHandler.Notes)
	api.POST("/charts/users", chartsHandler.Users)
	api.POST("/charts/drive", chartsHandler.Drive)
	api.POST("/charts/federation", chartsHandler.Federation)
	api.POST("/charts/instance", chartsHandler.Instance)
	api.POST("/charts/ap-request", chartsHandler.ApRequest)
	api.POST("/charts/active-users", chartsHandler.ActiveUsers)
	api.POST("/charts/user/notes", chartsHandler.UserNotes)
	api.POST("/charts/user/drive", chartsHandler.UserDrive)
	api.POST("/charts/user/following", chartsHandler.UserFollowing)
	api.POST("/charts/user/pv", chartsHandler.UserPv)
	api.POST("/charts/user/reactions", chartsHandler.UserReactions)

	// Following endpoints
	followingHandler := following.NewHandler(followingService, userService)
	api.POST("/following/create", followingHandler.Create, middleware.RequireAuth())
	api.POST("/following/delete", followingHandler.Delete, middleware.RequireAuth())
	api.POST("/following/requests/list", followingHandler.ListRequests, middleware.RequireAuth())
	api.POST("/following/requests/accept", followingHandler.AcceptRequest, middleware.RequireAuth())
	api.POST("/following/requests/reject", followingHandler.RejectRequest, middleware.RequireAuth())
	api.POST("/following/requests/cancel", followingHandler.CancelRequest, middleware.RequireAuth())

	// Public roles (Phase 6)
	rolesHandler := apiroles.NewHandler(roleService)
	api.POST("/roles/list", rolesHandler.List)
	api.POST("/roles/show", rolesHandler.Show)
	api.POST("/roles/users", rolesHandler.Users)

	// User lists (Phase 6)
	userListRepo := repository.NewUserListRepository(s.db)
	userListHandler := apiuserlists.NewHandler(userListRepo, idGen)
	api.POST("/users/lists/list", userListHandler.List, middleware.RequireAuth())
	api.POST("/users/lists/create", userListHandler.Create, middleware.RequireAuth())
	api.POST("/users/lists/show", userListHandler.Show, middleware.RequireAuth())
	api.POST("/users/lists/push", userListHandler.Push, middleware.RequireAuth())
	api.POST("/users/lists/pull", userListHandler.Pull, middleware.RequireAuth())
	api.POST("/users/lists/delete", userListHandler.Delete, middleware.RequireAuth())

	// Announcements (Phase 6)
	announcementRepo := repository.NewAnnouncementRepository(s.db)
	announcementHandler := apiannouncements.NewHandler(announcementRepo, idGen)
	api.POST("/announcements", announcementHandler.List)
	api.POST("/i/read-announcement", announcementHandler.ReadAnnouncement, middleware.RequireAuth())

	// Admin endpoints (Phase 5)
	abuseReportRepo := repository.NewAbuseReportRepository(s.db)
	modLogRepo := repository.NewModerationLogRepository(s.db)
	adminHandler := apiadmin.NewHandler(signupService, roleService, metaRepo, userRepo, idGen)
	adminHandler.SetAbuseRepo(abuseReportRepo)
	adminHandler.SetModLogRepo(modLogRepo)
	adminHandler.SetEmojiRepo(emojiRepo)
	api.POST("/admin/accounts/create", adminHandler.AccountsCreate)
	api.POST("/admin/show-user", adminHandler.ShowUser, middleware.RequireModerator(roleService))
	api.POST("/admin/show-users", adminHandler.ShowUsers, middleware.RequireModerator(roleService))
	api.POST("/admin/suspend-user", adminHandler.SuspendUser, middleware.RequireModerator(roleService))
	api.POST("/admin/unsuspend-user", adminHandler.UnsuspendUser, middleware.RequireModerator(roleService))
	api.POST("/admin/meta", adminHandler.AdminMeta, middleware.RequireAdmin(roleService))
	api.POST("/admin/update-meta", adminHandler.UpdateMeta, middleware.RequireAdmin(roleService))
	api.POST("/admin/roles/create", adminHandler.RolesCreate, middleware.RequireAdmin(roleService))
	api.POST("/admin/roles/show", adminHandler.RolesShow, middleware.RequireAdmin(roleService))
	api.POST("/admin/roles/list", adminHandler.RolesList, middleware.RequireAdmin(roleService))
	api.POST("/admin/roles/update", adminHandler.RolesUpdate, middleware.RequireAdmin(roleService))
	api.POST("/admin/roles/delete", adminHandler.RolesDelete, middleware.RequireAdmin(roleService))
	api.POST("/admin/roles/assign", adminHandler.RolesAssign, middleware.RequireModerator(roleService))
	api.POST("/admin/roles/unassign", adminHandler.RolesUnassign, middleware.RequireModerator(roleService))
	api.POST("/admin/roles/users", adminHandler.RolesUsers, middleware.RequireModerator(roleService))
	api.POST("/admin/roles/update-default-policies", adminHandler.RolesUpdateDefaultPolicies, middleware.RequireAdmin(roleService))
	api.POST("/admin/abuse-user-reports", adminHandler.AbuseReports, middleware.RequireModerator(roleService))
	api.POST("/admin/resolve-abuse-user-report", adminHandler.ResolveAbuseReport, middleware.RequireModerator(roleService))
	api.POST("/admin/show-moderation-logs", adminHandler.ShowModerationLogs, middleware.RequireModerator(roleService))
	api.POST("/admin/emoji/add", adminHandler.EmojiAdd, middleware.RequireAdmin(roleService))
	api.POST("/admin/emoji/update", adminHandler.EmojiUpdate, middleware.RequireAdmin(roleService))
	api.POST("/admin/emoji/delete", adminHandler.EmojiDelete, middleware.RequireAdmin(roleService))
	api.POST("/admin/emoji/list", adminHandler.EmojiList, middleware.RequireAdmin(roleService))
	api.POST("/admin/announcements/create", announcementHandler.AdminCreate, middleware.RequireAdmin(roleService))
	api.POST("/admin/announcements/update", announcementHandler.AdminUpdate, middleware.RequireAdmin(roleService))
	api.POST("/admin/announcements/delete", announcementHandler.AdminDelete, middleware.RequireAdmin(roleService))
	api.POST("/admin/announcements/list", announcementHandler.AdminList, middleware.RequireAdmin(roleService))

	// API catchall — 未実装エンドポイントに 200 を返してフロントエンドのクラッシュを防ぐ
	api.Any("/*", func(c echo.Context) error {
		slog.Warn("unimplemented API endpoint", "method", c.Request().Method, "path", c.Request().URL.Path)
		return c.JSON(http.StatusOK, map[string]any{})
	})

	// フロントエンドアセット配信
	// ビルド済みアセットがあれば静的配信、なければVite dev serverプロキシ
	frontendDir := FrontendDir()
	if _, err := os.Stat(frontendDir); err == nil {
		s.echo.Static("/vite", frontendDir)
	} else {
		s.echo.Any("/vite/*", newViteProxy("http://localhost:5173"))
	}

	// 静的アセット配信 (favicon, splash, icons等)
	staticDir := StaticDir()
	if _, err := os.Stat(staticDir); err == nil {
		s.echo.Static("/static-assets", staticDir)
		s.echo.File("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))
		s.echo.File("/apple-touch-icon.png", filepath.Join(staticDir, "apple-touch-icon.png"))
		s.echo.File("/robots.txt", filepath.Join(staticDir, "robots.txt"))
	}

	// Frontend HTML shell — SPA catchall (最後に登録)
	s.echo.GET("/*", frontendHTML(s.config, metaRepo))
}
