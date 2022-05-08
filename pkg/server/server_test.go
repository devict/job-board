package server_test

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
	"github.com/devict/job-board/pkg/server"
	"github.com/stretchr/testify/assert"
)

// Things to test:
// - job deleted after 30 days?

func TestIndex(t *testing.T) {
	s, _, dbmock := makeServer(t)
	defer s.Close()

	rows := sqlmock.NewRows(getDbFields(data.Job{})).
		AddRow(mockJobRow(data.Job{Position: "Pos 1"})...).
		AddRow(mockJobRow(data.Job{Position: "Pos 2"})...)
	dbmock.ExpectQuery(`SELECT \* FROM jobs`).WillReturnRows(rows)

	body, _ := sendRequest(t, s.URL)

	assert.Contains(t, string(body), "Pos 1")
	assert.Contains(t, string(body), "Pos 2")

	// TODO: What other assertions do we want to make about the home page?
}

func TestNewJob(t *testing.T) {
	s, _, _ := makeServer(t)
	defer s.Close()

	body, _ := sendRequest(t, fmt.Sprintf("%s/new", s.URL))

	// - assert that all the right fields are present
	tests := []struct {
		field    string
		required bool
		textArea bool
	}{
		{"position", true, false},
		{"organization", true, false},
		{"description", false, true},
		{"url", false, false},
		{"email", true, false},
	}

	for _, tt := range tests {
		reqStr := ""
		if tt.required {
			reqStr = "required.*"
		}

		base := "input"
		if tt.textArea {
			base = "textarea"
		}

		r := fmt.Sprintf(`<%s.+name="%s".*%s>`, base, tt.field, reqStr)
		assert.Regexp(t, regexp.MustCompile(r), string(body))
	}
}

func TestCreateJob(t *testing.T) {
	// TODO
	// - post some form data to the create job page
	// - assert the right sql insert query is called
	// - assert email sent
	// - assert slack and twitter posts
}

func TestViewJob(t *testing.T) {
	// TODO
	// - view job page
	// - assert right SQL query is called
	// - assert all the right data is displayed
}

func TestEditJobUnauthorized(t *testing.T) {
	// TODO
	// - attempt to go to edit job link
	// - assert unauthorized response
}

func TestUpdateJobUnauthorized(t *testing.T) {
	// TODO
	// - attempt to post to update job link
	// - assert unauthorized response
}

func TestEditJobAuthorized(t *testing.T) {
	// TODO
	// - go to edit job page with proper token
	// - assert the select sql query
	// - assert the fields and pre-populated values
}

func TestUpdateJobAuthorized(t *testing.T) {
	// TODO
	// - post data to update job route
	// - assert sql update query
	// - assert redirect to, wherever
}

// Helpers ------------------------------

type email struct {
	recipient, subject, body string
}

type mockService struct {
	emails []email
	tweets []data.Job
	slacks []data.Job
}

func (svc *mockService) SendEmail(recipient, subject, body string) error {
	svc.emails = append(svc.emails, email{recipient, subject, body})
	return nil
}

func (svc *mockService) PostToTwitter(job data.Job) error {
	svc.tweets = append(svc.tweets, job)
	return nil
}

func (svc *mockService) PostToSlack(job data.Job) error {
	svc.slacks = append(svc.slacks, job)
	return nil
}

func makeServer(t *testing.T) (*httptest.Server, *mockService, sqlmock.Sqlmock) {
	db, dbmock, err := sqlmock.New()
	assert.NoError(t, err)

	svc := &mockService{}
	s, err := server.NewServer(
		server.ServerConfig{
			Config:         config.Config{AppSecret: "sup"},
			DB:             db,
			EmailService:   svc,
			TwitterService: svc,
			SlackService:   svc,
			TemplatePath:   "../../templates",
		},
	)
	assert.NoError(t, err)

	testServer := httptest.NewServer(s.Handler)
	return testServer, svc, dbmock
}

func sendRequest(t *testing.T, path string) ([]byte, *http.Response) {
	resp, err := http.Get(path)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	return body, resp
}

func getDbFields(thing interface{}) []string {
	dbFields := make([]string, 0)

	t := reflect.TypeOf(thing)

	for i := 0; i < t.NumField(); i++ {
		dbTag := t.Field(i).Tag.Get("db")
		if dbTag != "" {
			dbFields = append(dbFields, dbTag)
		}
	}

	return dbFields
}

func mockJobRow(job data.Job) []driver.Value {
	vals := []driver.Value{
		"1",
		"A Position",
		"An Organization",
		sql.NullString{String: "https://devict.org", Valid: true},
		sql.NullString{},
		"example@example.com",
		time.Now(),
	}

	if job.ID != "" {
		vals[0] = job.ID
	}

	if job.Position != "" {
		vals[1] = job.Position
	}

	if job.Organization != "" {
		vals[2] = job.Organization
	}

	if job.Url.Valid {
		vals[3] = job.Url
	}

	if job.Description.Valid {
		vals[4] = job.Description
	}

	if job.Email != "" {
		vals[5] = job.Email
	}

	if !job.PublishedAt.IsZero() {
		vals[6] = job.PublishedAt
	}

	return vals
}
