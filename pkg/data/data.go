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
	ID           string         `db:"id"`
	Position     string         `db:"position"`
	Organization string         `db:"organization"`
	Url          sql.NullString `db:"url"`
	Description  sql.NullString `db:"description"`
	Email        string         `db:"email"`
	PublishedAt  time.Time      `db:"published_at"`
}

type Role struct {
	ID          string         `db:"id"`
	Name        string         `db:"name"`
	Email       string         `db:"email"`
	Phone       sql.NullString `db:"phone"`
	Role        string         `db:"role"`
	Resume      string         `db:"resume"`
	Linkedin    sql.NullString `db:"linkedin"`
	Website     sql.NullString `db:"website"`
	Github      sql.NullString `db:"github"`
	CompLow     sql.NullString `db:"comp_low"`
	CompHigh    sql.NullString `db:"comp_high"`
	PublishedAt time.Time      `db:"published_at"`
}

const (
	ErrNoPosition         = "Must provide a Position"
	ErrNoOrganization     = "Must provide a Organization"
	ErrNoEmail            = "Must provide an Email Address"
	ErrInvalidUrl         = "Must provide a valid Url"
	ErrInvalidEmail       = "Must provide a valid Email"
	ErrNoUrlOrDescription = "Must provide either a Url or a Description"
	ErrNoName             = "Must provide a Name"
	ErrNoRole             = "Must provide a Role"
	ErrNoResume           = "Must provide a Resume"
)

func (job *Job) Update(newParams NewJob) {
	job.Position = newParams.Position
	job.Organization = newParams.Organization

	job.Url.String = newParams.Url
	job.Url.Valid = newParams.Url != ""

	job.Description.String = newParams.Description
	job.Description.Valid = newParams.Description != ""
}

func (role *Role) Update(newParams NewRole) {
	role.Name = newParams.Name
	role.Role = newParams.Role
	role.Resume = newParams.Resume

	role.Phone.String = newParams.Phone
	role.Phone.Valid = newParams.Phone != ""

	role.Linkedin.String = newParams.Linkedin
	role.Linkedin.Valid = newParams.Linkedin != ""

	role.Website.String = newParams.Website
	role.Website.Valid = newParams.Website != ""

	role.Github.String = newParams.Github
	role.Github.Valid = newParams.Github != ""

	role.CompLow.String = newParams.CompLow
	role.CompLow.Valid = newParams.CompLow != ""

	role.CompHigh.String = newParams.CompHigh
	role.CompHigh.Valid = newParams.CompHigh != ""
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
		return "", fmt.Errorf("failed to convert job description to markdown (job id: %s): %w", job.ID, err)
	}

	return b.String(), nil
}

func (role *Role) RenderResume() (string, error) {
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
	if err := markdown.Convert([]byte(role.Resume), &b); err != nil {
		return "", fmt.Errorf("failed to convert resume to markdown (role id: %s): %w", role.ID, err)
	}

	return b.String(), nil
}

func (job *Job) Save(db *sqlx.DB) (sql.Result, error) {
	return db.Exec(
		"UPDATE jobs SET position = $1, organization = $2, url = $3, description = $4 WHERE id = $5",
		job.Position, job.Organization, job.Url, job.Description, job.ID,
	)
}

func (role *Role) Save(db *sqlx.DB) (sql.Result, error) {
	return db.Exec(
		"UPDATE roles SET name = $1, phone = $2, role = $3, resume = $4, linkedin = $5, website = $6, github = $7, comp_low = $8, comp_high = $9 WHERE id = $10",
		role.Name, role.Phone, role.Role, role.Resume, role.Linkedin, role.Website, role.Github, role.CompLow, role.CompHigh, role.ID,
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

func (role *Role) AuthSignature(secret string) string {
	input := fmt.Sprintf(
		"%s:%s:%s:%s",
		role.ID,
		role.Email,
		role.PublishedAt.String(),
		secret,
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

func GetAllRoles(db *sqlx.DB) ([]Role, error) {
	var roles []Role

	err := db.Select(&roles, "SELECT * FROM roles ORDER BY published_at DESC")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return roles, err
	}

	return roles, nil
}

func GetRole(id string, db *sqlx.DB) (Role, error) {
	var role Role

	err := db.Get(&role, "SELECT * FROM roles WHERE id = $1", id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return role, err
	}

	return role, nil
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

type NewRole struct {
	Name     string `form:"name"`
	Email    string `form:"email"`
	Phone    string `form:"phone"`
	Role     string `form:"role"`
	Resume   string `form:"resume"`
	Linkedin string `form:"linkedin"`
	Website  string `form:"website"`
	Github   string `form:"github"`
	CompLow  string `form:"complow"`
	CompHigh string `form:"comphigh"`
}

func (newJob *NewRole) Validate(update bool) map[string]string {
	errs := make(map[string]string)

	if newJob.Name == "" {
		errs["name"] = ErrNoName
	}

	if !update {
		if newJob.Email == "" {
			errs["email"] = ErrNoEmail
		} else if _, err := mail.ParseAddress(newJob.Email); err != nil {
			// TODO: Maybe do more than just validate email format?
			errs["email"] = ErrInvalidEmail
		}
	}

	if newJob.Role == "" {
		errs["role"] = ErrNoRole
	}

	if newJob.Resume == "" {
		errs["resume"] = ErrNoResume
	}

	if newJob.Linkedin != "" {
		if _, err := url.ParseRequestURI(newJob.Linkedin); err != nil {
			errs["linkedin"] = ErrInvalidUrl
		}
	}

	if newJob.Website != "" {
		if _, err := url.ParseRequestURI(newJob.Website); err != nil {
			errs["website"] = ErrInvalidUrl
		}
	}

	if newJob.Github != "" {
		if _, err := url.ParseRequestURI(newJob.Github); err != nil {
			errs["github"] = ErrInvalidUrl
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

func (newRole *NewRole) SaveToDB(db *sqlx.DB) (Role, error) {
	query := `INSERT INTO roles
    (name, email, phone, role, resume, linkedin, website, github, comp_low, comp_high)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    RETURNING *`

	params := []interface{}{
		newRole.Name,
		newRole.Email,
		sql.NullString{
			String: newRole.Phone,
			Valid:  newRole.Phone != "",
		},
		newRole.Role,
		newRole.Resume,
		sql.NullString{
			String: newRole.Linkedin,
			Valid:  newRole.Linkedin != "",
		},
		sql.NullString{
			String: newRole.Website,
			Valid:  newRole.Website != "",
		},
		sql.NullString{
			String: newRole.Github,
			Valid:  newRole.Github != "",
		},
		sql.NullString{
			String: newRole.CompLow,
			Valid:  newRole.CompLow != "",
		},
		sql.NullString{
			String: newRole.CompHigh,
			Valid:  newRole.CompHigh != "",
		},
	}

	var role Role
	if err := db.QueryRowx(query, params...).StructScan(&role); err != nil {
		return role, err
	}
	return role, nil
}
