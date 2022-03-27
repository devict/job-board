package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	log.SetOutput(os.Stderr)

	if err := run(); err != nil {
		log.Fatalln("main failed to run:", err)
	}

	log.Println("sucessful shutdown")
}

func run() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to LoadConfig: %w", err)
	}

	// migrate the db on startup
	m, err := migrate.New("file://sql", config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to migrate.New: %w", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to migrate Up: %w", err)
		}

		log.Println("no new migrations detected, schema is current")
	}

	db, err := sqlx.Open("postgres", config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to sqlx.Open: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("shutting down old jobs background process")
				return
			case <-ticker.C:
				_, err := db.Exec("DELETE FROM jobs WHERE published_at < NOW() - INTERVAL '30 DAYS'")
				if err != nil {
					log.Println(fmt.Errorf("error clearing old jobs: %w", err))
				}
			}
		}
	}()

	gin.SetMode(config.Env)
	gin.DefaultWriter = log.Writer()

	router := gin.Default()

	if err := router.SetTrustedProxies(nil); err != nil {
		return fmt.Errorf("failed to SetTrustedProxies: %w", err)
	}

	sessionOpts := sessions.Options{
		Path:     "/",
		MaxAge:   24 * 60, // 1 day
		Secure:   config.Env != "debug",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	sessionStore := cookie.NewStore([]byte(config.AppSecret))
	sessionStore.Options(sessionOpts)
	router.Use(sessions.Sessions("mysession", sessionStore))

	router.Static("/assets", "assets")
	router.HTMLRender = renderer()

	ctrl := &Controller{DB: db, Config: config}
	router.GET("/", ctrl.Index)
	router.GET("/new", ctrl.NewJob)
	router.POST("/jobs", ctrl.CreateJob)
	router.GET("/jobs/:id", ctrl.ViewJob)

	authorized := router.Group("/")
	authorized.Use(requireAuth(db, config.AppSecret))
	{
		authorized.GET("/jobs/:id/edit", ctrl.EditJob)
		authorized.POST("/jobs/:id", ctrl.UpdateJob)
	}

	server := http.Server{
		Addr:    config.Port,
		Handler: router,
	}

	serverErrors := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("server listening on port %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case <-serverErrors:
		return fmt.Errorf("received server error: %w", err)

	case sig := <-shutdown:
		log.Printf("received shutdown signal %q", sig)

		cancel()

		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to server.Shutdown: %w", err)
		}
	}

	wg.Wait()

	return nil
}

func renderer() multitemplate.Renderer {
	funcMap := template.FuncMap{
		"formatAsDate":          formatAsDate,
		"formatAsRfc3339String": formatAsRfc3339String,
	}

	r := multitemplate.NewRenderer()
	r.AddFromFilesFuncs("index", funcMap, "./templates/base.html", "./templates/index.html")
	r.AddFromFilesFuncs("new", funcMap, "./templates/base.html", "./templates/new.html")
	r.AddFromFilesFuncs("edit", funcMap, "./templates/base.html", "./templates/edit.html")
	r.AddFromFilesFuncs("view", funcMap, "./templates/base.html", "./templates/view.html")

	return r
}

func requireAuth(db *sqlx.DB, secret string) func(*gin.Context) {
	return func(ctx *gin.Context) {
		jobID := ctx.Param("id")
		job, err := getJob(jobID, db)
		if err != nil {
			log.Println(fmt.Errorf("requireAuth failed to getJob: %w", err))
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		token := ctx.Query("token")
		expected := signatureForJob(job, secret)

		if token != expected {
			ctx.AbortWithStatus(403)
			return
		}
	}
}
