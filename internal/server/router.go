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
	apiauth "github.com/shiroha-a/mk/internal/api/auth"
	"github.com/shiroha-a/mk/internal/api/blocking"
	apibubblegame "github.com/shiroha-a/mk/internal/api/bubblegame"
	apichannels "github.com/shiroha-a/mk/internal/api/channels"
	apicharts "github.com/shiroha-a/mk/internal/api/charts"
	apichat "github.com/shiroha-a/mk/internal/api/chat"
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
	apireversi "github.com/shiroha-a/mk/internal/api/reversi"
	apiroles "github.com/shiroha-a/mk/internal/api/roles"
	apisignin "github.com/shiroha-a/mk/internal/api/signin"
	"github.com/shiroha-a/mk/internal/api/streaming"
	apisw "github.com/shiroha-a/mk/internal/api/sw"
	apiuserlists "github.com/shiroha-a/mk/internal/api/userlists"
	"github.com/shiroha-a/mk/internal/api/users"
	apiwebhooks "github.com/shiroha-a/mk/internal/api/webhooks"
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
	noteCreateService.SetUserRepo(userRepo)
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
	notesHandler.SetDriveFileRepo(driveFileRepo)
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
	api.POST("/notes/search-by-tag", notesHandler.SearchByTag)
	api.POST("/notes/clips", notesHandler.Clips)
	api.POST("/notes/translate", notesHandler.Translate)
	api.POST("/notes/show-partial-bulk", notesHandler.ShowPartialBulk)
	// notes/drafts + notes/thread-muting + notes/polls/recommendation (実データ)
	notesHandler.SetDraftDB(s.db)
	api.POST("/notes/drafts/list", notesHandler.DraftsList, middleware.RequireAuth())
	api.POST("/notes/drafts/create", notesHandler.DraftsCreate, middleware.RequireAuth())
	api.POST("/notes/drafts/update", notesHandler.DraftsUpdate, middleware.RequireAuth())
	api.POST("/notes/drafts/delete", notesHandler.DraftsDelete, middleware.RequireAuth())
	api.POST("/notes/drafts/count", notesHandler.DraftsCount, middleware.RequireAuth())
	api.POST("/notes/thread-muting/create", notesHandler.ThreadMutingCreate, middleware.RequireAuth())
	api.POST("/notes/thread-muting/delete", notesHandler.ThreadMutingDelete, middleware.RequireAuth())
	api.POST("/notes/polls/recommendation", notesHandler.PollsRecommendation)

	// Users endpoints
	usersHandler := users.NewHandler(userService, followingService, noteRepo, idGen)
	usersHandler.SetChartHook(chartHooks)
	usersHandler.SetFollowingRepo(followingRepo)
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
	// Phase 7.3: users/* 完全化 (実データハンドラ)
	api.POST("/users/achievements", usersHandler.Achievements)
	api.POST("/users/clips", usersHandler.Clips)
	api.POST("/users/flashs", usersHandler.Flashs)
	api.POST("/users/gallery/posts", usersHandler.GalleryPosts)
	api.POST("/users/pages", usersHandler.Pages)
	api.POST("/users/get-frequently-replied-users", usersHandler.GetFrequentlyRepliedUsers)
	api.POST("/users/get-following-users-by-birthday", usersHandler.GetFollowingUsersByBirthday)
	api.POST("/users/recommendation", usersHandler.UserRecommendation)
	api.POST("/users/lists/get-memberships", usersHandler.ListsGetMemberships)
	api.POST("/users/lists/create-from-public", usersHandler.ListsCreateFromPublic, middleware.RequireAuth())
	api.POST("/users/lists/favorite", usersHandler.ListsFavorite, middleware.RequireAuth())
	api.POST("/users/lists/unfavorite", usersHandler.ListsUnfavorite, middleware.RequireAuth())
	api.POST("/users/lists/update", usersHandler.ListsUpdate, middleware.RequireAuth())
	api.POST("/users/lists/update-membership", usersHandler.ListsUpdateMembership, middleware.RequireAuth())

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
	api.POST("/i/regenerate-token", iHandler.RegenerateToken, middleware.RequireAuth())
	iHandler.SetFavoriteRepo(noteFavoriteRepo)

	// i/webhooks/* — Webhook管理 (実データ)
	webhookRepo := repository.NewWebhookRepository(s.db)
	webhookHandler := apiwebhooks.NewHandler(webhookRepo, idGen)
	api.POST("/i/webhooks/create", webhookHandler.Create, middleware.RequireAuth())
	api.POST("/i/webhooks/list", webhookHandler.List, middleware.RequireAuth())
	api.POST("/i/webhooks/show", webhookHandler.Show, middleware.RequireAuth())
	api.POST("/i/webhooks/update", webhookHandler.Update, middleware.RequireAuth())
	api.POST("/i/webhooks/delete", webhookHandler.Delete, middleware.RequireAuth())
	api.POST("/i/webhooks/test", webhookHandler.Test, middleware.RequireAuth())

	// Notifications endpoints
	notificationsHandler := notifications.NewHandler(notificationService, idGen)
	notificationsHandler.SetRepos(userRepo, noteRepo)
	api.POST("/i/notifications", notificationsHandler.Show, middleware.RequireAuth())
	api.POST("/i/notifications-grouped", notificationsHandler.Show, middleware.RequireAuth())
	api.POST("/notifications/mark-all-as-read", notificationsHandler.MarkAllAsRead, middleware.RequireAuth())
	api.POST("/notifications/create", notificationsHandler.Create, middleware.RequireAuth())
	api.POST("/notifications/flush", notificationsHandler.Flush, middleware.RequireAuth())
	api.POST("/notifications/test-notification", notificationsHandler.TestNotification, middleware.RequireAuth())

	// Phase 7.1a: フロントエンドが呼ぶ i/* スタブ
	api.POST("/i/claim-achievement", iHandler.ClaimAchievement, middleware.RequireAuth())
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

	// i/export-* — データエクスポート (asynqキューにエンキュー)
	for _, exportType := range []string{
		"notes", "following", "blocking", "mute", "favorites", "user-lists", "antennas", "clips",
	} {
		et := exportType
		api.POST("/i/export-"+et, func(c echo.Context) error {
			user := middleware.GetUser(c)
			if s.queueClient != nil {
				_ = s.queueClient.EnqueueExport(queue.ExportPayload{UserID: user.ID, Type: et})
			}
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireAuth())
	}
	// i/import-* — データインポート (asynqキューにエンキュー)
	for _, importType := range []string{
		"following", "blocking", "muting", "user-lists", "antennas",
	} {
		it := importType
		api.POST("/i/import-"+it, func(c echo.Context) error {
			user := middleware.GetUser(c)
			if s.queueClient != nil {
				_ = s.queueClient.EnqueueImport(queue.ImportPayload{UserID: user.ID, Type: it})
			}
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireAuth())
	}

	// Phase 7.1: i/* 完全化 (スタブ: フロントエンドの設定画面が動くように)
	// NoContent (204) を返すエンドポイント
	for _, ep := range []string{
		"i/update-email", "i/move",
		"i/2fa/register", "i/2fa/done", "i/2fa/unregister",
		"i/2fa/register-key", "i/2fa/key-done", "i/2fa/remove-key",
		"i/2fa/update-key", "i/2fa/password-less",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireAuth())
	}
	// 空配列を返すエンドポイント
	for _, ep := range []string{
		"i/gallery/likes", "i/gallery/posts", "i/page-likes",
		"i/registry/keys", "i/registry/scopes-with-domain",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.JSON(http.StatusOK, []any{})
		}, middleware.RequireAuth())
	}
	// オブジェクトを返すエンドポイント
	api.POST("/i/registry/get-detail", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{})
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
	driveHandler.SetRepos(driveFileRepo, driveFolderRepo, noteRepo)
	api.POST("/drive", driveHandler.Usage, middleware.RequireAuth())
	api.POST("/drive/files", driveHandler.FilesList, middleware.RequireAuth())
	api.POST("/drive/files/create", driveHandler.FilesCreate, middleware.RequireAuth())
	api.POST("/drive/files/show", driveHandler.FilesShow, middleware.RequireAuth())
	api.POST("/drive/files/update", driveHandler.FilesUpdate, middleware.RequireAuth())
	api.POST("/drive/files/delete", driveHandler.FilesDelete, middleware.RequireAuth())
	api.POST("/drive/files/find-by-hash", driveHandler.FilesFindByHash, middleware.RequireAuth())
	api.POST("/drive/folders/create", driveHandler.FoldersCreate, middleware.RequireAuth())
	api.POST("/drive/folders/show", driveHandler.FoldersShow, middleware.RequireAuth())
	api.POST("/drive/folders/update", driveHandler.FoldersUpdate, middleware.RequireAuth())
	api.POST("/drive/folders/delete", driveHandler.FoldersDelete, middleware.RequireAuth())
	api.POST("/drive/folders", driveHandler.FoldersList, middleware.RequireAuth())
	api.POST("/drive/folders/find", driveHandler.FoldersFind, middleware.RequireAuth())
	api.POST("/drive/files/find", driveHandler.FilesFind, middleware.RequireAuth())
	api.POST("/drive/files/check-existence", driveHandler.FilesCheckExistence, middleware.RequireAuth())
	api.POST("/drive/files/attached-notes", driveHandler.FilesAttachedNotes, middleware.RequireAuth())
	api.POST("/drive/files/attached-chat-messages", driveHandler.FilesAttachedChatMessages, middleware.RequireAuth())
	api.POST("/drive/files/upload-from-url", driveHandler.FilesUploadFromURL, middleware.RequireAuth())
	api.POST("/drive/files/move-bulk", driveHandler.FilesMoveBulk, middleware.RequireAuth())
	api.POST("/drive/stream", driveHandler.Stream, middleware.RequireAuth())
	// Phase 7.4: drive/* — 全て実データハンドラに移行済み

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
	// Phase 7.6: following/channels/clips/federation 残りエンドポイント
	for _, ep := range []string{
		"following/invalidate", "following/update", "following/update-all",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireAuth())
	}
	api.POST("/following/requests/sent", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAuth())
	// channels 残り
	for _, ep := range []string{
		"channels/favorite", "channels/unfavorite",
		"channels/mute/create", "channels/mute/delete",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireAuth())
	}
	for _, ep := range []string{
		"channels/my-favorites", "channels/mute/list",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.JSON(http.StatusOK, []any{})
		}, middleware.RequireAuth())
	}
	// clips 残り
	for _, ep := range []string{
		"clips/favorite", "clips/unfavorite",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireAuth())
	}
	api.POST("/clips/my-favorites", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAuth())
	// federation 残り
	for _, ep := range []string{
		"federation/followers", "federation/following",
		"federation/users",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.JSON(http.StatusOK, []any{})
		})
	}
	api.POST("/federation/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"topSubInstances": []any{}, "otherFollowersCount": 0,
			"topPubInstances": []any{}, "otherFollowingCount": 0,
		})
	})
	api.POST("/federation/update-remote-user", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, middleware.RequireAuth())

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

	// Phase 7.5: admin/* 残りエンドポイント一括追加
	// NoContent を返す admin エンドポイント (moderator)
	for _, ep := range []string{
		"admin/delete-account", "admin/delete-all-files-of-a-user",
		"admin/reset-password", "admin/send-email",
		"admin/unset-user-avatar", "admin/unset-user-banner",
		"admin/update-user-note", "admin/update-proxy-account",
		"admin/forward-abuse-user-report", "admin/update-abuse-user-report",
		"admin/accounts/delete",
		"admin/federation/delete-all-files",
		"admin/federation/refresh-remote-instance-metadata",
		"admin/federation/remove-all-following",
		"admin/federation/update-instance",
		"admin/drive/clean-remote-files", "admin/drive/cleanup",
		"admin/ad/create", "admin/ad/update", "admin/ad/delete",
		"admin/avatar-decorations/create", "admin/avatar-decorations/update",
		"admin/avatar-decorations/delete",
		"admin/promo/create",
		"admin/relays/add", "admin/relays/remove",
		"admin/invite/create",
		"admin/captcha/save",
		"admin/emoji/copy", "admin/emoji/import-zip",
		"admin/emoji/add-aliases-bulk", "admin/emoji/remove-aliases-bulk",
		"admin/emoji/set-aliases-bulk", "admin/emoji/set-category-bulk",
		"admin/emoji/set-license-bulk", "admin/emoji/delete-bulk",
		"admin/system-webhook/create", "admin/system-webhook/update",
		"admin/system-webhook/delete", "admin/system-webhook/test",
		"admin/abuse-report/notification-recipient/create",
		"admin/abuse-report/notification-recipient/update",
		"admin/abuse-report/notification-recipient/delete",
		"admin/queue/clear", "admin/queue/promote-jobs",
		"admin/queue/remove-job", "admin/queue/retry-job",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		}, middleware.RequireModerator(roleService))
	}
	// 空配列・オブジェクトを返す admin エンドポイント
	for _, ep := range []string{
		"admin/ad/list",
		"admin/avatar-decorations/list",
		"admin/drive/files",
		"admin/emoji/list-remote",
		"admin/invite/list",
		"admin/relays/list",
		"admin/system-webhook/list",
		"admin/abuse-report/notification-recipient/list",
		"admin/queue/deliver-delayed", "admin/queue/inbox-delayed",
		"admin/queue/jobs",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.JSON(http.StatusOK, []any{})
		}, middleware.RequireModerator(roleService))
	}
	// オブジェクトを返す admin エンドポイント
	api.POST("/admin/accounts/find-by-email", func(c echo.Context) error {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "USER_NOT_FOUND", "message": "User not found.", "id": "a]504947-b888-4a99-9f62-8c4a0f3a3dab"},
		})
	}, middleware.RequireModerator(roleService))
	api.POST("/admin/get-user-ips", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireModerator(roleService))
	api.POST("/admin/get-index-stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/get-table-stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/server-info", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"machine": "misskey-go", "os": "linux", "node": "n/a",
			"cpu": map[string]any{"model": "unknown", "cores": 0},
			"mem": map[string]any{"total": 0},
			"fs":  map[string]any{"total": 0, "used": 0},
		})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/captcha/current", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"provider": nil})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/drive/show-file", func(c echo.Context) error {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "NO_SUCH_FILE", "message": "No such file.", "id": "ac4f7b11-1a6e-47e3-bf3d-3dce9a0e07ab"},
		})
	}, middleware.RequireModerator(roleService))
	for _, ep := range []string{
		"admin/system-webhook/show",
		"admin/abuse-report/notification-recipient/show",
	} {
		ep := ep
		api.POST("/"+ep, func(c echo.Context) error {
			return c.JSON(http.StatusNotFound, map[string]any{
				"error": map[string]any{"code": "NOT_FOUND", "message": "Not found.", "id": "00000000-0000-0000-0000-000000000000"},
			})
		}, middleware.RequireModerator(roleService))
	}
	api.POST("/admin/queue/queues", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/queue/queue-stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/queue/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"deliver": map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
			"inbox":   map[string]any{"activeSince": nil, "active": 0, "waiting": 0, "delayed": 0},
		})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/queue/show-job", func(c echo.Context) error {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "NOT_FOUND", "message": "Not found."},
		})
	}, middleware.RequireAdmin(roleService))
	api.POST("/admin/queue/show-job-logs", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	}, middleware.RequireAdmin(roleService))

	// --- Phase 7.6b: chat/*, auth/*, ap/*, sw/*, reversi/*, bubble-game/*, misc ---

	// announcements/show — 個別アナウンスメント取得
	api.POST("/announcements/show", func(c echo.Context) error {
		var req struct {
			AnnouncementID string `json:"announcementId"`
		}
		if err := c.Bind(&req); err != nil || req.AnnouncementID == "" {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": map[string]any{"code": "INVALID_PARAM", "message": "announcementId is required."},
			})
		}
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": map[string]any{"code": "NO_SUCH_ANNOUNCEMENT", "message": "No such announcement.", "id": "e0ef2076-ee94-4ebc-a2c8-63bec9e7ab72"},
		})
	})

	// auth/* — MiAuth/OAuth セッション
	authSessionRepo := repository.NewAuthSessionRepository(s.db)
	authHandler := apiauth.NewHandler(authSessionRepo, s.config, idGen)
	api.POST("/auth/session/generate", authHandler.SessionGenerate)
	api.POST("/auth/session/show", authHandler.SessionShow)
	api.POST("/auth/session/userkey", authHandler.SessionUserkey)
	api.POST("/auth/accept", authHandler.Accept, middleware.RequireAuth())

	// ap/* — ActivityPub API lookup (実データ: ローカルオブジェクト解決)
	api.POST("/ap/get", apHandler.APIGet, middleware.RequireAdmin(roleService))
	api.POST("/ap/show", apHandler.APIShow, middleware.RequireAuth())
	api.POST("/ap/notes", func(c echo.Context) error {
		return c.JSON(http.StatusOK, []any{})
	})

	// sw/* — Service Worker push notifications (実データ)
	swSubRepo := repository.NewSwSubscriptionRepository(s.db)
	swHandler := apisw.NewHandler(swSubRepo, metaRepo, idGen)
	api.POST("/sw/register", swHandler.Register, middleware.RequireAuth())
	api.POST("/sw/show-registration", swHandler.ShowRegistration, middleware.RequireAuth())
	api.POST("/sw/update-registration", swHandler.UpdateRegistration, middleware.RequireAuth())
	api.POST("/sw/unregister", swHandler.Unregister)

	// reversi/* — オセロゲーム (実データ)
	reversiRepo := repository.NewReversiRepository(s.db)
	reversiHandler := apireversi.NewHandler(reversiRepo, idGen)
	api.POST("/reversi/games", reversiHandler.Games)
	api.POST("/reversi/invitations", reversiHandler.Invitations, middleware.RequireAuth())
	api.POST("/reversi/show-game", reversiHandler.ShowGame)
	api.POST("/reversi/match", reversiHandler.Match, middleware.RequireAuth())
	api.POST("/reversi/cancel-match", reversiHandler.CancelMatch, middleware.RequireAuth())
	api.POST("/reversi/surrender", reversiHandler.Surrender, middleware.RequireAuth())
	api.POST("/reversi/verify", reversiHandler.Verify)

	// bubble-game/* — バブルゲーム (実データ)
	bubbleGameRepo := repository.NewBubbleGameRepository(s.db)
	bubbleGameHandler := apibubblegame.NewHandler(bubbleGameRepo, idGen)
	api.POST("/bubble-game/register", bubbleGameHandler.Register, middleware.RequireAuth())
	api.POST("/bubble-game/ranking", bubbleGameHandler.Ranking)

	// chat/* — Misskey v2026 チャット機能 (実データ)
	chatRepo := repository.NewChatRepository(s.db)
	chatHandler := apichat.NewHandler(chatRepo, idGen)
	api.POST("/chat/rooms/create", chatHandler.RoomsCreate, middleware.RequireAuth())
	api.POST("/chat/rooms/show", chatHandler.RoomsShow, middleware.RequireAuth())
	api.POST("/chat/rooms/update", chatHandler.RoomsUpdate, middleware.RequireAuth())
	api.POST("/chat/rooms/delete", chatHandler.RoomsDelete, middleware.RequireAuth())
	api.POST("/chat/rooms/owned", chatHandler.RoomsOwned, middleware.RequireAuth())
	api.POST("/chat/rooms/joined", chatHandler.RoomsJoined, middleware.RequireAuth())
	api.POST("/chat/rooms/leave", chatHandler.RoomsLeave, middleware.RequireAuth())
	api.POST("/chat/rooms/mute", chatHandler.RoomsMute, middleware.RequireAuth())
	api.POST("/chat/rooms/unmute", chatHandler.RoomsUnmute, middleware.RequireAuth())
	api.POST("/chat/rooms/transfer-ownership", chatHandler.RoomsTransferOwnership, middleware.RequireAuth())
	api.POST("/chat/rooms/invitations/create", chatHandler.InvitationsCreate, middleware.RequireAuth())
	api.POST("/chat/rooms/invitations/delete", chatHandler.InvitationsDelete, middleware.RequireAuth())
	api.POST("/chat/rooms/invitations/accept", chatHandler.InvitationsAccept, middleware.RequireAuth())
	api.POST("/chat/rooms/invitations/reject", chatHandler.InvitationsReject, middleware.RequireAuth())
	api.POST("/chat/rooms/members/ban", chatHandler.MembersBan, middleware.RequireAuth())
	api.POST("/chat/rooms/members/update-membership", chatHandler.MembersUpdateMembership, middleware.RequireAuth())
	api.POST("/chat/messages", chatHandler.Messages, middleware.RequireAuth())
	api.POST("/chat/messages/create", chatHandler.MessagesCreate, middleware.RequireAuth())
	api.POST("/chat/messages/show", chatHandler.MessagesShow, middleware.RequireAuth())
	api.POST("/chat/messages/update", chatHandler.MessagesUpdate, middleware.RequireAuth())
	api.POST("/chat/messages/delete", chatHandler.MessagesDelete, middleware.RequireAuth())
	api.POST("/chat/messages/read", chatHandler.MessagesRead, middleware.RequireAuth())
	api.POST("/chat/messages/search", chatHandler.MessagesSearch, middleware.RequireAuth())
	api.POST("/chat/messages/reactions/create", chatHandler.ReactionsCreate, middleware.RequireAuth())
	api.POST("/chat/messages/reactions/delete", chatHandler.ReactionsDelete, middleware.RequireAuth())
	api.POST("/chat/history", chatHandler.History, middleware.RequireAuth())
	api.POST("/chat/unread-count", chatHandler.UnreadCount, middleware.RequireAuth())

	// --- その他の残りエンドポイント ---
	// test — フロントエンドのテスト用
	api.POST("/test", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{})
	})

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

	// フロントエンド配布アセット (locales, fonts等)
	frontendDistDir := FrontendDistDir()
	if _, err := os.Stat(frontendDistDir); err == nil {
		s.echo.Static("/assets", frontendDistDir)
	}

	// twemoji SVG配信
	twemojiDir := TwemojiDir()
	if _, err := os.Stat(twemojiDir); err == nil {
		s.echo.Static("/twemoji", twemojiDir)
	}

	// client-assets配信 (バブルゲーム、フラッシュ等のフロントエンドアセット)
	clientAssetsDir := ClientAssetsDir()
	if _, err := os.Stat(clientAssetsDir); err == nil {
		s.echo.Static("/client-assets", clientAssetsDir)
	}

	// 静的アセット配信 (favicon, splash, icons等)
	staticDir := StaticDir()
	if _, err := os.Stat(staticDir); err == nil {
		s.echo.Static("/static-assets", staticDir)
		s.echo.File("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))
		s.echo.File("/apple-touch-icon.png", filepath.Join(staticDir, "apple-touch-icon.png"))
		s.echo.File("/robots.txt", filepath.Join(staticDir, "robots.txt"))
	}

	// identicon — ユーザーアイコン自動生成
	s.echo.GET("/identicon/:x", identiconHandler())

	// manifest.json — PWA用
	s.echo.GET("/manifest.json", manifestJSON(s.config, metaRepo))

	// Frontend HTML shell — SPA catchall (最後に登録)
	s.echo.GET("/*", frontendHTML(s.config, metaRepo))
}
