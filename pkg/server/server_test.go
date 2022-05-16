package server_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	s, _, dbmock, _ := makeServer(t)
	defer s.Close()

	rows := sqlmock.NewRows(getDbFields(data.Job{})).
		AddRow(mockJobRow(data.Job{Position: "Pos 1"})...).
		AddRow(mockJobRow(data.Job{Position: "Pos 2"})...)
	dbmock.ExpectQuery(`SELECT \* FROM jobs`).WillReturnRows(rows)

	body, _ := sendRequest(t, s.URL, nil)

	assert.Contains(t, string(body), "Pos 1")
	assert.Contains(t, string(body), "Pos 2")

	// TODO: What other assertions do we want to make about the home page?
}

func TestNewJob(t *testing.T) {
	s, _, _, _ := makeServer(t)
	defer s.Close()

	body, _ := sendRequest(t, fmt.Sprintf("%s/new", s.URL), nil)

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
		assert.Regexp(t, regexp.MustCompile(r), body)
	}
}

func TestCreateJob(t *testing.T) {
	s, svcmock, dbmock, conf := makeServer(t)
	defer s.Close()

	tests := []struct {
		values        map[string][]string
		expectSuccess bool
		// TODO: what else should I expect?
	}{
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {""},
				"url":          {"https://devict.org"},
				"email":        {"test@example.com"},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {"Super rad place to work"},
				"url":          {""},
				"email":        {"test@example.com"},
			},
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		newJob := data.Job{
			ID:           "1",
			Position:     tt.values["position"][0],
			Organization: tt.values["organization"][0],
			Description:  sql.NullString{String: tt.values["description"][0], Valid: true},
			Url:          sql.NullString{String: tt.values["url"][0], Valid: true},
			Email:        tt.values["email"][0],
			PublishedAt:  time.Now(),
		}

		if tt.expectSuccess {
			dbmock.ExpectQuery(`INSERT INTO jobs`).WillReturnRows(
				sqlmock.NewRows(getDbFields(data.Job{})).AddRow(mockJobRow(newJob)...),
			)

			expectHomeQuery(dbmock, []data.Job{newJob})
		}

		reqBody := url.Values(tt.values).Encode()
		_, resp := sendRequest(t, fmt.Sprintf("%s%s", s.URL, "/jobs"), []byte(reqBody))

		assert.Equal(t, 200, resp.StatusCode)

		if tt.expectSuccess {
			expectHomeQuery(dbmock, []data.Job{newJob})

			homeBody, _ := sendRequest(t, s.URL, nil)
			assert.Contains(t, homeBody, tt.values["position"][0])
			assert.Contains(t, homeBody, tt.values["organization"][0])

			assert.Equal(t, 1, len(svcmock.emails))
			assert.Equal(t, 1, len(svcmock.tweets))
			assert.Equal(t, 1, len(svcmock.slacks))

			assert.Equal(t, "Job Created!", svcmock.emails[0].subject)
			assert.Equal(t, tt.values["email"][0], svcmock.emails[0].recipient)
			assert.Contains(t, svcmock.emails[0].body, server.SignedJobRoute(newJob, conf))

			assert.Contains(t, svcmock.tweets, newJob)
			assert.Contains(t, svcmock.slacks, newJob)
		} else {
			// TODO: failure scenario
		}

		resetServiceMock(svcmock)
	}

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

func makeServer(t *testing.T) (*httptest.Server, *mockService, sqlmock.Sqlmock, *config.Config) {
	db, dbmock, err := sqlmock.New()
	assert.NoError(t, err)

	conf := &config.Config{AppSecret: "sup"}

	svc := &mockService{}
	s, err := server.NewServer(
		&server.ServerConfig{
			Config:         conf,
			DB:             db,
			EmailService:   svc,
			TwitterService: svc,
			SlackService:   svc,
			TemplatePath:   "../../templates",
		},
	)
	assert.NoError(t, err)

	testServer := httptest.NewServer(s.Handler)

	conf.URL = testServer.URL

	return testServer, svc, dbmock, conf
}

func sendRequest(t *testing.T, path string, postBody []byte) (string, *http.Response) {
	var resp *http.Response
	var err error

	if postBody == nil {
		resp, err = http.Get(path)
	} else {
		resp, err = http.Post(path, "application/x-www-form-urlencoded", bytes.NewReader(postBody))
	}

	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, 200)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	return string(body), resp
}

func resetServiceMock(svc *mockService) {
	svc.emails = []email{}
	svc.tweets = []data.Job{}
	svc.slacks = []data.Job{}
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

func expectHomeQuery(dbmock sqlmock.Sqlmock, jobs []data.Job) {
	rows := sqlmock.NewRows(getDbFields(data.Job{}))
	for _, job := range jobs {
		rows.AddRow(mockJobRow(job)...)
	}
	dbmock.ExpectQuery(`SELECT \* FROM jobs`).WillReturnRows(rows)
}
