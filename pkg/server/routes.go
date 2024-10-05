package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
	"github.com/devict/job-board/pkg/services"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Controller struct {
	DB             *sqlx.DB
	EmailService   services.IEmailService
	SlackService   services.ISlackService
	TwitterService services.ITwitterService
	Config         *config.Config
}

func (ctrl *Controller) Index(ctx *gin.Context) {
	jobs, err := data.GetAllJobs(ctrl.DB)
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

func (ctrl *Controller) JobsJSON(ctx *gin.Context) {
	jobs, err := data.GetAllJobs(ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("JobsJSON failed to getAllJobs: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	ctx.Header("Content-Type", "application/json")
	ctx.JSON(200, gin.H{"items": jobs})
}

func (ctrl *Controller) About(ctx *gin.Context) {
	ctx.HTML(200, "about", gin.H{})
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
	job, err := data.GetJob(id, ctrl.DB)
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
	var newJobInput data.NewJob
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

	if errs := newJobInput.Validate(false); len(errs) != 0 {
		for k, v := range errs {
			session.AddFlash(v, fmt.Sprintf("%s_err", k))
		}

		ctx.Redirect(302, "/new")
		return
	}

	job, err := newJobInput.SaveToDB(ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("failed to save job to db: %w", err))
		session.AddFlash("Error creating job")
		ctx.Redirect(302, "/new")
		return
	}

	if ctrl.EmailService != nil {
		// TODO: make this a nicer html template?
		subject := fmt.Sprintf("Job %s Created!", newJobInput.Position)
		message := fmt.Sprintf(`
			<!doctype html>
			<html>
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Job Created</title>
				<style>
					body { font-family: Arial, sans-serif; background-color: #f4f4f4; padding: 20px; }
					.container { max-width: 600px; margin: 0 auto; background-color: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 0 10px rgba(0, 0, 0, 0.1); }
					h1 { color: #dc7900; }
					p { font-size: 16px; line-height: 1.6; color: #555; }
					a { color: #007BFF; text-decoration: none; }
					a:hover { text-decoration: underline; }
					.footer { margin-top: 20px; font-size: 12px; color: #999; text-align: center; }
				</style>
			</head>
			<body>
				<div class="container">
					<h1>Job %s Created!</h1>
					<p>Congratulations! Your job posting for the position <strong>%s</strong> has been successfully created.</p>
					<p>You can edit or update your job posting by clicking the link below:</p>
					<p><a href="%s">Edit Job Posting</a></p>
					<div class="footer">
						<p>&copy; <a href="https://jobs.devict.org/">Job Board</a></p>
					</div>
				</div>
			</body>
			</html>
		`, newJobInput.Position, newJobInput.Position, SignedJobRoute(job, ctrl.Config))

		err = ctrl.EmailService.SendEmail(newJobInput.Email, subject, message)
		if err != nil {
			log.Println(fmt.Errorf("failed to sendEmail: %w", err))
			// continuing...
		}
	}

	if ctrl.SlackService != nil {
		if err = ctrl.SlackService.PostToSlack(job); err != nil {
			log.Println(fmt.Errorf("failed to postToSlack: %w", err))
			// continuing...
		}
	}

	if ctrl.TwitterService != nil {
		if err = ctrl.TwitterService.PostToTwitter(job); err != nil {
			log.Println(fmt.Errorf("failed to postToTwitter: %w", err))
			// continuing...
		}
	}

	session.AddFlash("Job created!")
	ctx.Redirect(302, "/")
}

func (ctrl *Controller) UpdateJob(ctx *gin.Context) {
	id := ctx.Param("id")

	var newJobInput data.NewJob
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

	if errs := newJobInput.Validate(true); len(errs) != 0 {
		for k, v := range errs {
			session.AddFlash(v, fmt.Sprintf("%s_err", k))
		}

		token := ctx.Query("token")
		// TODO: somehow preserve previously provided values?
		ctx.Redirect(302, fmt.Sprintf("/jobs/%s/edit?token=%s", id, token))
		return
	}

	job, err := data.GetJob(id, ctrl.DB)
	if err != nil {
		log.Println(fmt.Errorf("failed to getJob: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	job.Update(newJobInput)
	if _, err = job.Save(ctrl.DB); err != nil {
		log.Println(fmt.Errorf("failed to job.save: %w", err))
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	session.AddFlash("Job updated!")
	ctx.Redirect(302, "/")
}

func (ctrl *Controller) ViewJob(ctx *gin.Context) {
	id := ctx.Param("id")
	job, err := data.GetJob(id, ctrl.DB)
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
