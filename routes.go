package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Controller struct {
	DB     *sqlx.DB
	Config Config
}

func (ctrl *Controller) Index(ctx *gin.Context) {
	jobs, err := getAllJobs(ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("Index failed to getAllJobs: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	ctx.HTML(200, "index", addFlash(ctx, gin.H{
		"jobs":   jobs,
		"noJobs": len(jobs) == 0,
	}))
}

func (ctrl *Controller) NewJob(ctx *gin.Context) {
	session := sessions.Default(ctx)

	fields := []string{"position", "organization", "url", "description", "email"}

	tVars := gin.H{}
	for _, k := range fields {
		f := fmt.Sprintf("%s_err", k)
		tVars[f] = session.Flashes(f)
	}

	ctx.HTML(200, "new", addFlash(ctx, tVars))
}

func (ctrl *Controller) EditJob(ctx *gin.Context) {
	session := sessions.Default(ctx)

	id := ctx.Param("id")
	job, err := getJob(id, ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("failed to getJob: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	token := ctx.Query("token")
	tVars := gin.H{"job": job, "token": token}

	fields := []string{"position", "organization", "url", "description", "email"}
	for _, k := range fields {
		f := fmt.Sprintf("%s_err", k)
		tVars[f] = session.Flashes(f)
	}

	ctx.HTML(200, "edit", addFlash(ctx, tVars))
}

func (ctrl *Controller) CreateJob(ctx *gin.Context) {
	var newJobInput NewJob
	if err := ctx.Bind(&newJobInput); err != nil {
		log.Println(fmt.Errorf("failed to ctx.Bind: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	session := sessions.Default(ctx)
	defer func() {
		if err := session.Save(); err != nil {
			log.Println(fmt.Errorf("CreateJob failed to session.Save: %w", err))
		}
	}()

	if errs := newJobInput.validate(false); len(errs) != 0 {
		for k, v := range errs {
			session.AddFlash(v, fmt.Sprintf("%s_err", k))
		}

		ctx.Redirect(302, "/new")
		return
	}

	job, err := newJobInput.saveToDB(ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("failed to save job to db: %w", err))
		session.AddFlash("Error creating job")
		ctx.Redirect(302, "/new")
		return
	}

	if ctrl.Config.Email.SMTPHost != "" {
		// TODO: make this a nicer html template?
		message := fmt.Sprintf(
			"Your job has been created!\n\n<a href=\"%s\">Use this link to edit the job posting</a>",
			signedJobRoute(job, ctrl.Config),
		)
		err = sendEmail(newJobInput.Email, "Job Created!", message, ctrl.Config.Email)
		if err != nil {
			log.Println(fmt.Errorf("failed to sendEmail: %w", err))
			// continuing...
		}
	}

	if ctrl.Config.SlackHook != "" {
		if err = postToSlack(job, ctrl.Config); err != nil {
			log.Println(fmt.Errorf("failed to postToSlack: %w", err))
			// continuing...
		}
	}

	if ctrl.Config.Twitter.AccessToken != "" {
		if err = postToTwitter(job, ctrl.Config); err != nil {
			log.Println(fmt.Errorf("failed to postToTwitter: %w", err))
			// continuing...
		}
	}

	session.AddFlash("Job created!")
	ctx.Redirect(302, "/")
}

func (ctrl *Controller) UpdateJob(ctx *gin.Context) {
	id := ctx.Param("id")

	var newJobInput NewJob
	if err := ctx.Bind(&newJobInput); err != nil {
		log.Println(fmt.Errorf("failed to ctx.Bind: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	session := sessions.Default(ctx)
	defer func() {
		if err := session.Save(); err != nil {
			log.Println(fmt.Errorf("failed to session.Save: %w", err))
		}
	}()

	if errs := newJobInput.validate(true); len(errs) != 0 {
		for k, v := range errs {
			session.AddFlash(v, fmt.Sprintf("%s_err", k))
		}

		// TODO: somehow preserve previously provided values?
		ctx.Redirect(302, "/jobs/"+id+"/edit")
		return
	}

	job, err := getJob(id, ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("failed to getJob: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	job.update(newJobInput)
	if _, err = job.save(ctrl.DB); err != nil {
		log.Println(fmt.Errorf("failed to job.save: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	session.AddFlash("Job updated!")
	ctx.Redirect(302, "/")
}

func (ctrl *Controller) ViewJob(ctx *gin.Context) {
	id := ctx.Param("id")
	job, err := getJob(id, ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("failed to getJob: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	description, err := job.RenderDescription()
	if err != nil {
		log.Println(fmt.Errorf("failed to render job description as markdown: %w", err))
		description = job.Description.String
		// continuing...
	}

	ctx.HTML(200, "view", gin.H{"job": job, "description": template.HTML(description)})
}

func addFlash(ctx *gin.Context, base gin.H) gin.H {
	session := sessions.Default(ctx)
	base["flashes"] = session.Flashes()
	session.Save()
	return base
}
