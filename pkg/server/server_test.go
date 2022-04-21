package server_test

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
	"github.com/devict/job-board/pkg/server"
	"github.com/stretchr/testify/assert"
)

// Things to test:
// - create a new job
// - view a job
// - view all jobs
// - update a job
// - unauthorized access to job edit (view and post)
// - job deleted after 30 days?

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
	s, err := server.NewServer(config.Config{AppSecret: "sup"}, db, svc, svc, svc, "../../templates")
	assert.NoError(t, err)

	testServer := httptest.NewServer(s.Handler)
	return testServer, svc, dbmock
}

func TestIndex(t *testing.T) {
	s, _, dbmock := makeServer(t)
	defer s.Close()

	rows := sqlmock.NewRows(getDbFields(data.Job{})).
		AddRow(mockJobRow(data.Job{Position: "Pos 1"})...).
		AddRow(mockJobRow(data.Job{Position: "Pos 2"})...)
	dbmock.ExpectQuery(`SELECT \* FROM jobs`).WillReturnRows(rows)

	// make request for index route
	resp, err := http.Get(s.URL)
	assert.NoError(t, err)

	assert.Equal(t, resp.StatusCode, 200)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	assert.Contains(t, string(body), "Pos 1")
	assert.Contains(t, string(body), "Pos 2")
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
