package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/activitypub"
	"github.com/shiroha-a/mk/internal/api/antennas"
	"github.com/shiroha-a/mk/internal/api/ap"
	"github.com/shiroha-a/mk/internal/api/blocking"
	apichannels "github.com/shiroha-a/mk/internal/api/channels"
	"github.com/shiroha-a/mk/internal/api/drive"
	apifederation "github.com/shiroha-a/mk/internal/api/federation"
	"github.com/shiroha-a/mk/internal/api/following"
	"github.com/shiroha-a/mk/internal/api/i"
	"github.com/shiroha-a/mk/internal/api/inbox"
	"github.com/shiroha-a/mk/internal/api/meta"
	"github.com/shiroha-a/mk/internal/api/mute"
	"github.com/shiroha-a/mk/internal/api/nodeinfo"
	"github.com/shiroha-a/mk/internal/api/notes"
	"github.com/shiroha-a/mk/internal/api/notifications"
	"github.com/shiroha-a/mk/internal/api/renotemute"
	"github.com/shiroha-a/mk/internal/api/streaming"
	"github.com/shiroha-a/mk/internal/api/users"
	"github.com/shiroha-a/mk/internal/api/wellknown"
	coreantenna "github.com/shiroha-a/mk/internal/core/antenna"
	coreblocking "github.com/shiroha-a/mk/internal/core/blocking"
	corechannel "github.com/shiroha-a/mk/internal/core/channel"
	coredrive "github.com/shiroha-a/mk/internal/core/drive"
	"github.com/shiroha-a/mk/internal/core/event"
	corefederation "github.com/shiroha-a/mk/internal/core/federation"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	coreinstance "github.com/shiroha-a/mk/internal/core/instance"
	coremuting "github.com/shiroha-a/mk/internal/core/muting"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	corenotification "github.com/shiroha-a/mk/internal/core/notification"
	corepoll "github.com/shiroha-a/mk/internal/core/poll"
	corereaction "github.com/shiroha-a/mk/internal/core/reaction"
	coretimeline "github.com/shiroha-a/mk/internal/core/timeline"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/queue"
	"github.com/shiroha-a/mk/internal/queue/processors"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
	"github.com/shiroha-a/mk/internal/stream"
	"github.com/shiroha-a/mk/internal/stream/channels"
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

	// Core services
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

	// Drive (LocalStorage)
	driveStorage := coredrive.NewLocalStorage("./drive-files", s.config.DriveURL)
	driveService := coredrive.NewService(driveFileRepo, driveFolderRepo, driveStorage, idGen)

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

	// Health check
	s.echo.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api := s.echo.Group("/api")

	// Meta endpoint (public)
	metaHandler := meta.NewHandler(s.config, metaRepo)
	api.POST("/meta", metaHandler.Meta)
	api.POST("/ping", metaHandler.Ping)

	// Notes endpoints
	notesHandler := notes.NewHandler(noteRepo, noteCreateService, noteDeleteService, noteQueryService, timelineService, reactionService, pollService, idGen)
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

	// Users endpoints
	usersHandler := users.NewHandler(userService, followingService, noteRepo, idGen)
	api.POST("/users/show", usersHandler.Show)
	api.POST("/users/search", usersHandler.Search)
	api.POST("/users/notes", usersHandler.Notes)
	api.POST("/users/followers", usersHandler.Followers)
	api.POST("/users/following", usersHandler.Following)

	// Account endpoints
	iHandler := i.NewHandler(userService, idGen)
	api.POST("/i", iHandler.Me, middleware.RequireAuth())
	api.POST("/i/update", iHandler.Update, middleware.RequireAuth())
	api.POST("/i/pin", iHandler.Pin, middleware.RequireAuth())
	api.POST("/i/unpin", iHandler.Unpin, middleware.RequireAuth())

	// Notifications endpoints
	notificationsHandler := notifications.NewHandler(notificationService, idGen)
	api.POST("/i/notifications", notificationsHandler.Show, middleware.RequireAuth())
	api.POST("/notifications/mark-all-as-read", notificationsHandler.MarkAllAsRead, middleware.RequireAuth())

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
	s.echo.GET("/files/:accessKey", func(c echo.Context) error {
		key := c.Param("accessKey")
		body, err := driveStorage.Get(key)
		if err != nil {
			return c.NoContent(http.StatusNotFound)
		}
		defer body.Close()
		return c.Stream(http.StatusOK, "application/octet-stream", body)
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
	s.echo.POST("/inbox", inboxHandler.Inbox)
	s.echo.POST("/users/:id/inbox", inboxHandler.Inbox)

	// Federation endpoints
	federationHandler := apifederation.NewHandler(instanceService)
	api.POST("/federation/instances", federationHandler.Instances)
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

	// Following endpoints
	followingHandler := following.NewHandler(followingService, userService)
	api.POST("/following/create", followingHandler.Create, middleware.RequireAuth())
	api.POST("/following/delete", followingHandler.Delete, middleware.RequireAuth())
	api.POST("/following/requests/list", followingHandler.ListRequests, middleware.RequireAuth())
	api.POST("/following/requests/accept", followingHandler.AcceptRequest, middleware.RequireAuth())
	api.POST("/following/requests/reject", followingHandler.RejectRequest, middleware.RequireAuth())
	api.POST("/following/requests/cancel", followingHandler.CancelRequest, middleware.RequireAuth())
}
