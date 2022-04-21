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

func NewServer(
	c config.Config,
	db *sql.DB,
	emailService services.IEmailService,
	twitterService services.ITwitterService,
	slackService services.ISlackService,
	templatePath string,
) (http.Server, error) {
	gin.SetMode(c.Env)
	gin.DefaultWriter = log.Writer()

	router := gin.Default()

	if err := router.SetTrustedProxies(nil); err != nil {
		return http.Server{}, fmt.Errorf("failed to SetTrustedProxies: %w", err)
	}

	sessionOpts := sessions.Options{
		Path:     "/",
		MaxAge:   24 * 60, // 1 day
		Secure:   c.Env != "debug",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	sessionStore := cookie.NewStore([]byte(c.AppSecret))
	sessionStore.Options(sessionOpts)
	router.Use(sessions.Sessions("mysession", sessionStore))

	router.Static("/assets", "assets")
	router.HTMLRender = renderer(templatePath)

	sqlxDb := sqlx.NewDb(db, "postgres")

	ctrl := &Controller{DB: sqlxDb, Config: c}
	router.GET("/", ctrl.Index)
	router.GET("/new", ctrl.NewJob)
	router.POST("/jobs", ctrl.CreateJob)
	router.GET("/jobs/:id", ctrl.ViewJob)

	authorized := router.Group("/")
	authorized.Use(requireAuth(sqlxDb, c.AppSecret))
	{
		authorized.GET("/jobs/:id/edit", ctrl.EditJob)
		authorized.POST("/jobs/:id", ctrl.UpdateJob)
	}

	return http.Server{
		Addr:    c.Port,
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
	r.AddFromFilesFuncs("new", funcMap, basePath, path.Join(templatePath, "new.html"))
	r.AddFromFilesFuncs("edit", funcMap, basePath, path.Join(templatePath, "edit.html"))
	r.AddFromFilesFuncs("view", funcMap, basePath, path.Join(templatePath, "view.html"))

	return r
}

func requireAuth(db *sqlx.DB, secret string) func(*gin.Context) {
	return func(ctx *gin.Context) {
		jobID := ctx.Param("id")
		job, err := data.GetJob(jobID, db)
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
