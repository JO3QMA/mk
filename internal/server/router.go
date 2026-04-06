package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/misskey-dev/misskey-go/internal/api/i"
	"github.com/misskey-dev/misskey-go/internal/api/meta"
	"github.com/misskey-dev/misskey-go/internal/api/notes"
	"github.com/misskey-dev/misskey-go/internal/api/users"
	"github.com/misskey-dev/misskey-go/internal/misc/id"
	"github.com/misskey-dev/misskey-go/internal/repository"
	"github.com/misskey-dev/misskey-go/internal/server/middleware"
)

func (s *Server) setupRoutes() {
	idGen, err := id.NewGenerator(s.config.ID)
	if err != nil {
		idGen, _ = id.NewGenerator("aidx")
	}

	userRepo := repository.NewUserRepository(s.db)
	noteRepo := repository.NewNoteRepository(s.db)
	metaRepo := repository.NewMetaRepository(s.db)
	pollRepo := repository.NewPollRepository(s.db)

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
	notesHandler := notes.NewHandler(noteRepo, pollRepo, idGen)
	api.POST("/notes/create", notesHandler.Create, middleware.RequireAuth())
	api.POST("/notes/show", notesHandler.Show)
	api.POST("/notes/delete", notesHandler.Delete, middleware.RequireAuth())

	// Users endpoints
	usersHandler := users.NewHandler(userRepo)
	api.POST("/users/show", usersHandler.Show)

	// Account endpoints
	iHandler := i.NewHandler(userRepo)
	api.POST("/i", iHandler.Me, middleware.RequireAuth())
}
