package data

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
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
	ID              string         `db:"id" json:"id"`
	Position        string         `db:"position" json:"position"`
	Organization    string         `db:"organization" json:"organization"`
	Url             sql.NullString `db:"url" json:"-"`
	JSONUrl         *string        `json:"url"`
	Description     sql.NullString `db:"description" json:"-"`
	DescriptionJSON *string        `json:"description"`
	Email           string         `db:"email" json:"email"`
	PublishedAt     time.Time      `db:"published_at" json:"published_at"`
}

const (
	ErrNoPosition         = "Must provide a Position"
	ErrNoOrganization     = "Must provide a Organization"
	ErrNoEmail            = "Must provide an Email Address"
	ErrInvalidUrl         = "Must provide a valid Url"
	ErrInvalidEmail       = "Must provide a valid Email"
	ErrNoUrlOrDescription = "Must provide either a Url or a Description"
)

func (job *Job) Update(newParams NewJob) {
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

func (job *Job) Save(db *sqlx.DB) (sql.Result, error) {
	return db.Exec(
		"UPDATE jobs SET position = $1, organization = $2, url = $3, description = $4 WHERE id = $5",
		job.Position, job.Organization, job.JSONUrl, job.DescriptionJSON, job.ID,
	)
}

func (job *Job) AuthSignature(secret string) string {
	input := fmt.Sprintf(
		"%s:%s:%s",
		job.ID,
		job.Email,
		job.PublishedAt.String(),
	)

	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(input))

	return base64.URLEncoding.EncodeToString(hash.Sum(nil))
}

func GetAllJobs(db *sqlx.DB) ([]Job, error) {
	var jobs []Job

	err := db.Select(&jobs, "SELECT * FROM jobs ORDER BY published_at DESC")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return jobs, err
	}

	for i := range jobs {
		if !jobs[i].Url.Valid {
			jobs[i].JSONUrl = &jobs[i].Url.String
		}
		if !jobs[i].Description.Valid {
			jobs[i].DescriptionJSON = &jobs[i].Description.String
		}
	}

	return jobs, nil
}

func GetJob(id string, db *sqlx.DB) (Job, error) {
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

func (newJob *NewJob) Validate(update bool) map[string]string {
	errs := make(map[string]string)

	if newJob.Position == "" {
		errs["position"] = ErrNoPosition
	}

	if newJob.Organization == "" {
		errs["organization"] = ErrNoOrganization
	}

	if newJob.Url == "" && newJob.Description == "" {
		errs["url"] = ErrNoUrlOrDescription
	} else if newJob.Description == "" {
		if _, err := url.ParseRequestURI(newJob.Url); err != nil {
			errs["url"] = ErrInvalidUrl
		}
	}

	if !update {
		if newJob.Email == "" {
			errs["email"] = ErrNoEmail
		} else if _, err := mail.ParseAddress(newJob.Email); err != nil {
			// TODO: Maybe do more than just validate email format?
			errs["email"] = ErrInvalidEmail
		}
	}

	return errs
}

func (newJob *NewJob) SaveToDB(db *sqlx.DB) (Job, error) {
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
