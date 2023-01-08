package server

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
	"github.com/devict/job-board/pkg/services"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

const JobRoute = "job"

type ServerConfig struct {
	Config         *config.Config
	DB             *sql.DB
	EmailService   services.IEmailService
	TwitterService services.ITwitterService
	SlackService   services.ISlackService
	TemplatePath   string
}

func NewServer(c *ServerConfig) (http.Server, error) {
	gin.SetMode(c.Config.Env)
	gin.DefaultWriter = log.Writer()

	router := gin.Default()

	if err := router.SetTrustedProxies(nil); err != nil {
		return http.Server{}, fmt.Errorf("failed to SetTrustedProxies: %w", err)
	}

	sessionOpts := sessions.Options{
		Path:     "/",
		MaxAge:   24 * 60, // 1 day
		Secure:   c.Config.Env != "debug",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	sessionStore := cookie.NewStore([]byte(c.Config.AppSecret))
	sessionStore.Options(sessionOpts)
	router.Use(sessions.Sessions("mysession", sessionStore))

	router.Static("/assets", "assets")
	router.HTMLRender = renderer(c.TemplatePath)

	sqlxDb := sqlx.NewDb(c.DB, "postgres")

	ctrl := &Controller{
		DB:             sqlxDb,
		Config:         c.Config,
		EmailService:   c.EmailService,
		SlackService:   c.SlackService,
		TwitterService: c.TwitterService,
	}
	router.GET("/", ctrl.Index)
	router.GET("/about", ctrl.About)
	router.GET("/new", ctrl.NewJob)
	router.POST("/jobs", ctrl.CreateJob)
	router.GET("/jobs/:id", ctrl.ViewJob)

	authorized := router.Group("/")
	authorized.Use(requireTokenAuth(sqlxDb, c.Config.AppSecret, JobRoute))
	{
		authorized.GET("/jobs/:id/edit", ctrl.EditJob)
		authorized.POST("/jobs/:id", ctrl.UpdateJob)
	}

	return http.Server{
		Addr:    c.Config.Port,
		Handler: router,
	}, nil
}

func renderer(templatePath string) multitemplate.Renderer {
	funcMap := template.FuncMap{
		"formatAsDate":          formatAsDate,
		"formatAsRfc3339String": formatAsRfc3339String,
	}

	basePath := path.Join(templatePath, "base.html")

	r := multitemplate.NewRenderer()
	r.AddFromFilesFuncs("index", funcMap, basePath, path.Join(templatePath, "index.html"))
	r.AddFromFilesFuncs("about", funcMap, basePath, path.Join(templatePath, "about.html"))
	r.AddFromFilesFuncs("new", funcMap, basePath, path.Join(templatePath, "new.html"))
	r.AddFromFilesFuncs("edit", funcMap, basePath, path.Join(templatePath, "edit.html"))
	r.AddFromFilesFuncs("view", funcMap, basePath, path.Join(templatePath, "view.html"))

	return r
}

func requireTokenAuth(db *sqlx.DB, secret, authType string) func(*gin.Context) {
	return func(ctx *gin.Context) {
		var expected string

		switch authType {
		case JobRoute:
			jobID := ctx.Param("id")
			job, err := data.GetJob(jobID, db)
			if err != nil {
				log.Println(fmt.Errorf("requiretokenauth failed to getjob: %w", err))
				ctx.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if job.ID != jobID {
				log.Println(fmt.Errorf("requiretokenauth failed to find job with getjob: %w", err))
				ctx.AbortWithStatus(http.StatusNotFound)
				return
			}
			expected = job.AuthSignature(secret)
		default:
			log.Println("requireTokenAuth failed, unexpected authType:", authType)
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		token := ctx.Query("token")

		// This is the same if it is a job or a user
		if token != expected {
			ctx.AbortWithStatus(403)
			return
		}
	}
}
