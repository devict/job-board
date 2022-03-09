package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

type Job struct {
	ID           string         `db:"id"`
	Position     string         `db:"position"`
	Organization string         `db:"organization"`
	Url          sql.NullString `db:"url"`
	Description  sql.NullString `db:"description"`
	Email        string         `db:"email"`
	PublishedAt  time.Time      `db:"published_at"`
}

func (job *Job) update(newParams NewJob) {
	job.Position = newParams.Position
	job.Organization = newParams.Organization

	job.Url.String = newParams.Url
	job.Url.Valid = newParams.Url != ""

	job.Description.String = newParams.Description
	job.Description.Valid = newParams.Description != ""
}

func (job *Job) RenderDescription() (string, error) {
	if !job.Description.Valid {
		return "", nil
	}

	markdown := goldmark.New(
		goldmark.WithExtensions(
			extension.NewLinkify(
				extension.WithLinkifyAllowedProtocols([][]byte{
					[]byte("http:"),
					[]byte("https:"),
				}),
			),
		),
	)

	var b bytes.Buffer
	if err := markdown.Convert([]byte(job.Description.String), &b); err != nil {
		return "", fmt.Errorf("failed to convert job descroption to markdown (job id: %s): %w", job.ID, err)
	}

	return b.String(), nil
}

func (job *Job) save(db *sqlx.DB) (sql.Result, error) {
	return db.Exec(
		"UPDATE jobs SET position = $1, organization = $2, url = $3, description = $4 WHERE id = $5",
		job.Position, job.Organization, job.Url, job.Description, job.ID,
	)
}

func getAllJobs(db *sqlx.DB) ([]Job, error) {
	var jobs []Job

	err := db.Select(&jobs, "SELECT * FROM jobs")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return jobs, err
	}

	return jobs, nil
}

func getJob(id string, db *sqlx.DB) (Job, error) {
	var job Job

	err := db.Get(&job, "SELECT * FROM jobs WHERE id = $1", id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return job, err
	}

	return job, nil
}

type NewJob struct {
	Position     string `form:"position"`
	Organization string `form:"organization"`
	Url          string `form:"url"`
	Description  string `form:"description"`
	Email        string `form:"email"`
}

func (newJob *NewJob) validate(update bool) map[string]string {
	errs := make(map[string]string)

	if newJob.Position == "" {
		errs["position"] = "Must provide a Position"
	}

	if newJob.Organization == "" {
		errs["organization"] = "Must provide a Organization"
	}

	if newJob.Url == "" && newJob.Description == "" {
		errs["url"] = "Must provide either a Url or a Description"
	}

	if newJob.Description == "" {
		if _, err := url.ParseRequestURI(newJob.Url); err != nil {
			errs["url"] = "Must provide a valid Url"
		}
	}

	if !update {
		if newJob.Email == "" {
			errs["email"] = "Must provide an Email Address"
		}

		// TODO: Maybe do more than just validate email format?
		if _, err := mail.ParseAddress(newJob.Email); err != nil {
			errs["email"] = "Must provide a valid Email"
		}
	}

	return errs
}

func (newJob *NewJob) saveToDB(db *sqlx.DB) (Job, error) {
	query := `INSERT INTO jobs
    (position, organization, url, description, email)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *`

	params := []interface{}{
		newJob.Position,
		newJob.Organization,
		sql.NullString{
			String: newJob.Url,
			Valid:  newJob.Url != "",
		},
		sql.NullString{
			String: newJob.Description,
			Valid:  newJob.Description != "",
		},
		newJob.Email,
	}

	var job Job
	if err := db.QueryRowx(query, params...).StructScan(&job); err != nil {
		return job, err
	}
	return job, nil
}
