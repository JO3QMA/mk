package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/shiroha-a/mk/internal/api/following"
	"github.com/shiroha-a/mk/internal/api/i"
	"github.com/shiroha-a/mk/internal/api/meta"
	"github.com/shiroha-a/mk/internal/api/notes"
	"github.com/shiroha-a/mk/internal/api/users"
	corefollowing "github.com/shiroha-a/mk/internal/core/following"
	corenote "github.com/shiroha-a/mk/internal/core/note"
	coreuser "github.com/shiroha-a/mk/internal/core/user"
	"github.com/shiroha-a/mk/internal/misc/id"
	"github.com/shiroha-a/mk/internal/repository"
	"github.com/shiroha-a/mk/internal/server/middleware"
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

	// Core services
	noteCreateService := corenote.NewCreateService(noteRepo, pollRepo, idGen)
	noteDeleteService := corenote.NewDeleteService(noteRepo)
	userService := coreuser.NewService(userRepo)
	followingService := corefollowing.NewService(userRepo, followingRepo, followRequestRepo, idGen)

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
	notesHandler := notes.NewHandler(noteRepo, noteCreateService, noteDeleteService, idGen)
	api.POST("/notes/create", notesHandler.Create, middleware.RequireAuth())
	api.POST("/notes/show", notesHandler.Show)
	api.POST("/notes/delete", notesHandler.Delete, middleware.RequireAuth())

	// Users endpoints
	usersHandler := users.NewHandler(userService)
	api.POST("/users/show", usersHandler.Show)

	// Account endpoints
	iHandler := i.NewHandler(userService)
	api.POST("/i", iHandler.Me, middleware.RequireAuth())

	// Following endpoints
	followingHandler := following.NewHandler(followingService, userService)
	api.POST("/following/create", followingHandler.Create, middleware.RequireAuth())
	api.POST("/following/delete", followingHandler.Delete, middleware.RequireAuth())
	api.POST("/following/requests/list", followingHandler.ListRequests, middleware.RequireAuth())
	api.POST("/following/requests/accept", followingHandler.AcceptRequest, middleware.RequireAuth())
	api.POST("/following/requests/reject", followingHandler.RejectRequest, middleware.RequireAuth())
	api.POST("/following/requests/cancel", followingHandler.CancelRequest, middleware.RequireAuth())
}
