package server_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
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
	"golang.org/x/net/publicsuffix"
)

// TODO: Things to test:
// - job deleted after 30 days?

func TestIndex(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)
	defer s.Close()

	expectSelectJobsQuery(dbmock, []data.Job{
		{Position: "Pos 1"},
		{Position: "Pos 2"},
	})

	expectSelectRolesQuery(dbmock, []data.Role{
		{Name: "Foo Bar"},
		{Name: "Baz Qux"},
	})

	body, _ := sendRequest(t, s.URL, nil)

	assert.Contains(t, string(body), "Pos 1")
	assert.Contains(t, string(body), "Pos 2")
	assert.Contains(t, string(body), "Foo Bar")
	assert.Contains(t, string(body), "Baz Qux")

	// TODO: What other assertions do we want to make about the home page?
}

func TestNewJob(t *testing.T) {
	s, _, _, _ := makeServer(t)
	defer s.Close()

	body, resp := sendRequest(t, fmt.Sprintf("%s/new", s.URL), nil)

	assert.Equal(t, 200, resp.StatusCode)

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

func TestNewRole(t *testing.T) {
	s, _, _, _ := makeServer(t)
	defer s.Close()

	body, resp := sendRequest(t, fmt.Sprintf("%s/newrole", s.URL), nil)

	assert.Equal(t, 200, resp.StatusCode)

	// - assert that all the right fields are present
	tests := []struct {
		field    string
		required bool
		textArea bool
	}{
		{"name", true, false},
		{"email", true, false},
		{"phone", false, false},
		{"role", true, false},
		{"resume", true, true},
		{"linkedin", false, false},
		{"website", false, false},
		{"github", false, false},
		{"complow", false, false},
		{"comphigh", false, false},
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
		values            map[string][]string
		expectSuccess     bool
		expectErrMessages []string
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
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {""},
				"url":          {""},
				"email":        {"test@example.com"},
			},
			expectSuccess:     false,
			expectErrMessages: []string{data.ErrNoUrlOrDescription},
		},
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {"Cool cool cool"},
				"url":          {""},
				"email":        {""},
			},
			expectSuccess:     false,
			expectErrMessages: []string{data.ErrNoEmail},
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

			expectSelectJobsQuery(dbmock, []data.Job{newJob})
			expectSelectRolesQuery(dbmock, []data.Role{})
		}

		reqBody := url.Values(tt.values).Encode()
		respBody, resp := sendRequest(t, fmt.Sprintf("%s/jobs", s.URL), []byte(reqBody))

		// Should follow the redirect and result in a 200 regardless of success/failure
		assert.Equal(t, 200, resp.StatusCode)

		if tt.expectSuccess {
			assert.Contains(t, respBody, tt.values["position"][0])
			assert.Contains(t, respBody, tt.values["organization"][0])

			assert.Equal(t, 1, len(svcmock.emails))
			assert.Equal(t, 1, len(svcmock.tweets))
			assert.Equal(t, 1, len(svcmock.jobSlacks))
			assert.Equal(t, 0, len(svcmock.roleSlacks))

			assert.Equal(t, "Job Created!", svcmock.emails[0].subject)
			assert.Equal(t, tt.values["email"][0], svcmock.emails[0].recipient)
			assert.Contains(t, svcmock.emails[0].body, server.SignedJobRoute(newJob, conf))

			assert.Contains(t, svcmock.tweets, newJob)
			assert.Contains(t, svcmock.jobSlacks, newJob)
		} else {
			for _, errMsg := range tt.expectErrMessages {
				assert.Contains(t, respBody, errMsg)
			}
			assert.Empty(t, svcmock.emails)
			assert.Empty(t, svcmock.tweets)
			assert.Empty(t, svcmock.jobSlacks)
			assert.Empty(t, svcmock.roleSlacks)
		}

		resetServiceMock(svcmock)
	}
}

func TestCreateRole(t *testing.T) {
	s, svcmock, dbmock, conf := makeServer(t)
	defer s.Close()

	tests := []struct {
		values            map[string][]string
		expectSuccess     bool
		expectErrMessages []string
	}{
		{
			values: map[string][]string{
				"name":     {"Test McTestington"},
				"email":    {"test@example.com"},
				"phone":    {"316-555-1111"},
				"role":     {"role 1"},
				"resume":   {"neat"},
				"linkedin": {"https://www.linkedin.com/example"},
				"website":  {""},
				"github":   {""},
				"complow":  {""},
				"comphigh": {""},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"name":     {"Test McTestington"},
				"email":    {"test@example.com"},
				"phone":    {""},
				"role":     {"role 2"},
				"resume":   {"wow"},
				"linkedin": {""},
				"website":  {""},
				"github":   {""},
				"complow":  {""},
				"comphigh": {""},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"name":     {"Test McTestington"},
				"email":    {"testexample.com"},
				"phone":    {""},
				"role":     {"role 1"},
				"resume":   {"cool"},
				"linkedin": {""},
				"website":  {""},
				"github":   {""},
				"complow":  {""},
				"comphigh": {""},
			},
			expectSuccess:     false,
			expectErrMessages: []string{data.ErrInvalidEmail},
		},
	}

	for _, tt := range tests {
		newRole := data.Role{
			ID:          "1",
			Name:        tt.values["name"][0],
			Email:       tt.values["email"][0],
			Phone:       sql.NullString{String: tt.values["phone"][0], Valid: tt.values["phone"][0] != ""},
			Role:        tt.values["role"][0],
			Resume:      tt.values["resume"][0],
			Linkedin:    sql.NullString{String: tt.values["linkedin"][0], Valid: tt.values["linkedin"][0] != ""},
			Website:     sql.NullString{String: tt.values["website"][0], Valid: tt.values["website"][0] != ""},
			Github:      sql.NullString{String: tt.values["github"][0], Valid: tt.values["github"][0] != ""},
			CompLow:     sql.NullString{String: tt.values["complow"][0], Valid: tt.values["complow"][0] != ""},
			CompHigh:    sql.NullString{String: tt.values["comphigh"][0], Valid: tt.values["comphigh"][0] != ""},
			PublishedAt: time.Now(),
		}

		if tt.expectSuccess {
			dbmock.ExpectQuery(`INSERT INTO roles`).WillReturnRows(
				sqlmock.NewRows(getDbFields(data.Role{})).AddRow(mockRoleRow(newRole)...),
			)

			expectSelectJobsQuery(dbmock, []data.Job{})
			expectSelectRolesQuery(dbmock, []data.Role{newRole})
		}

		reqBody := url.Values(tt.values).Encode()
		respBody, resp := sendRequest(t, fmt.Sprintf("%s/roles", s.URL), []byte(reqBody))

		// Should follow the redirect and result in a 200 regardless of success/failure
		assert.Equal(t, 200, resp.StatusCode)

		if tt.expectSuccess {
			assert.Contains(t, respBody, tt.values["name"][0])
			assert.Contains(t, respBody, tt.values["role"][0])

			assert.Equal(t, 1, len(svcmock.emails))
			assert.Equal(t, 0, len(svcmock.tweets))
			assert.Equal(t, 0, len(svcmock.jobSlacks))
			assert.Equal(t, 1, len(svcmock.roleSlacks))

			assert.Equal(t, "Post Created!", svcmock.emails[0].subject)
			assert.Equal(t, tt.values["email"][0], svcmock.emails[0].recipient)
			assert.Contains(t, svcmock.emails[0].body, server.SignedRoleRoute(newRole, conf))
			assert.Contains(t, svcmock.roleSlacks, newRole)
		} else {
			for _, errMsg := range tt.expectErrMessages {
				assert.Contains(t, respBody, errMsg)
			}
			assert.Empty(t, svcmock.emails)
			assert.Empty(t, svcmock.tweets)
			assert.Empty(t, svcmock.jobSlacks)
			assert.Empty(t, svcmock.roleSlacks)
		}

		resetServiceMock(svcmock)
	}
}

func TestViewJob(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)
	defer s.Close()

	tests := []struct {
		job data.Job
	}{
		{
			job: data.Job{
				ID:           "1",
				Position:     "Pos",
				Organization: "Org",
				Description:  sql.NullString{String: "Coolest job eva", Valid: true},
				Url:          sql.NullString{},
				Email:        "test@example.com",
			},
		},
		{
			job: data.Job{
				ID:           "2",
				Position:     "Pos 2",
				Organization: "Org 2",
				Description:  sql.NullString{},
				Url:          sql.NullString{String: "https://devict.org", Valid: true},
				Email:        "test@example.com",
			},
		},
	}

	for _, tt := range tests {
		expectGetJobQuery(dbmock, tt.job)

		respBody, resp := sendRequest(t, fmt.Sprintf("%s/jobs/%s", s.URL, tt.job.ID), nil)

		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, respBody, tt.job.Position)
		assert.Contains(t, respBody, tt.job.Organization)

		if tt.job.Description.Valid {
			assert.Contains(t, respBody, tt.job.Description.String)
		}

		if tt.job.Url.Valid {
			assert.Contains(t, respBody, tt.job.Url.String)
		}

		assert.NotContains(t, respBody, tt.job.Email) // Don't expose the email!
	}
}

func TestViewRole(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)
	defer s.Close()

	tests := []struct {
		role data.Role
	}{
		{
			role: data.Role{
				ID:       "1",
				Name:     "Test Testly",
				Email:    "test@example.com",
				Phone:    sql.NullString{String: "316-555-1515", Valid: true},
				Role:     "Super cool role",
				Resume:   "# hey\n\nhire this guy",
				Linkedin: sql.NullString{String: "https://www.linkedin.com/in/example", Valid: true},
				Website:  sql.NullString{},
				Github:   sql.NullString{},
				CompLow:  sql.NullString{},
				CompHigh: sql.NullString{},
			},
		},
		{
			role: data.Role{
				ID:       "2",
				Name:     "Not Test Testly",
				Email:    "test2@example.com",
				Phone:    sql.NullString{},
				Role:     "Also cool role",
				Resume:   "# too cool\n\nfor linkedin",
				Linkedin: sql.NullString{},
				Website:  sql.NullString{},
				Github:   sql.NullString{},
				CompLow:  sql.NullString{},
				CompHigh: sql.NullString{},
			},
		},
	}

	for _, tt := range tests {
		expectGetRoleQuery(dbmock, tt.role)

		respBody, resp := sendRequest(t, fmt.Sprintf("%s/roles/%s", s.URL, tt.role.ID), nil)

		assert.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, respBody, tt.role.Name)
		assert.Contains(t, respBody, tt.role.Role)

		if tt.role.Phone.Valid {
			assert.Contains(t, respBody, tt.role.Phone.String)
		}

		if tt.role.Linkedin.Valid {
			assert.Contains(t, respBody, tt.role.Linkedin.String)
		}
	}
}

func TestEditJobUnauthorized(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)

	job := data.Job{ID: "1", PublishedAt: time.Now()}

	expectGetJobQuery(dbmock, job)

	_, resp := sendRequest(t, fmt.Sprintf("%s/jobs/%s/edit?token=incorrect", s.URL, job.ID), nil)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestEditRoleUnauthorized(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)

	role := data.Role{ID: "1", PublishedAt: time.Now()}

	expectGetRoleQuery(dbmock, role)

	_, resp := sendRequest(t, fmt.Sprintf("%s/roles/%s/edit?token=incorrect", s.URL, role.ID), nil)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestUpdateJobUnauthorized(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)

	job := data.Job{ID: "1", PublishedAt: time.Now()}

	expectGetJobQuery(dbmock, job)

	_, resp := sendRequest(t, fmt.Sprintf("%s/jobs/%s?token=incorrect", s.URL, job.ID), []byte("daaaata"))
	assert.Equal(t, 403, resp.StatusCode)
}

func TestUpdateRoleUnauthorized(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)

	role := data.Role{ID: "1", PublishedAt: time.Now()}

	expectGetRoleQuery(dbmock, role)

	_, resp := sendRequest(t, fmt.Sprintf("%s/roles/%s?token=incorrect", s.URL, role.ID), []byte("daaaata"))
	assert.Equal(t, 403, resp.StatusCode)
}

func TestUpdateJobDoesNotExist(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)

	job := data.Job{ID: "1", PublishedAt: time.Now()}

	expectGetJobQuery(dbmock, job)

	_, resp := sendRequest(t, fmt.Sprintf("%s/jobs/%s?token=incorrect", s.URL, "32"), []byte("daaaata"))
	assert.Equal(t, 404, resp.StatusCode)
}

func TestUpdateRoleDoesNotExist(t *testing.T) {
	s, _, dbmock, _ := makeServer(t)

	role := data.Role{ID: "1", PublishedAt: time.Now()}

	expectGetRoleQuery(dbmock, role)

	_, resp := sendRequest(t, fmt.Sprintf("%s/roles/%s?token=incorrect", s.URL, "32"), []byte("daaaata"))
	assert.Equal(t, 404, resp.StatusCode)
}

func TestEditJobAuthorized(t *testing.T) {
	_, _, dbmock, conf := makeServer(t)

	job := data.Job{
		Position:     "A position",
		Organization: "An organization",
		Description:  sql.NullString{String: "A description", Valid: true},
		ID:           "1",
		Email:        "secret@secret.com",
		PublishedAt:  time.Now(),
	}

	// Query executes twice, once for the auth middleware, and
	// a second time for the actual route
	expectGetJobQuery(dbmock, job)
	expectGetJobQuery(dbmock, job)

	signedEditRoute := server.SignedJobRoute(job, conf)
	respBody, resp := sendRequest(t, signedEditRoute, nil)

	assert.Equal(t, 200, resp.StatusCode)

	assert.Regexp(t, fmt.Sprintf(`<input.+name="position".*value="%s".*>`, job.Position), respBody)
	assert.Regexp(t, fmt.Sprintf(`<input.+name="organization".*value="%s".*>`, job.Organization), respBody)
	assert.Regexp(t, fmt.Sprintf(`<textarea.+name="description".*>%s</textarea>`, job.Description.String), respBody)
}

func TestEditRoleAuthorized(t *testing.T) {
	_, _, dbmock, conf := makeServer(t)

	role := data.Role{
		ID:          "1",
		Name:        "Test",
		Email:       "test@example.com",
		Phone:       sql.NullString{String: "316-555-1515", Valid: true},
		Role:        "any",
		Resume:      "# hey \n\n yo",
		Linkedin:    sql.NullString{},
		Website:     sql.NullString{},
		Github:      sql.NullString{},
		CompLow:     sql.NullString{},
		CompHigh:    sql.NullString{},
		PublishedAt: time.Now(),
	}

	// Query executes twice, once for the auth middleware, and
	// a second time for the actual route
	expectGetRoleQuery(dbmock, role)
	expectGetRoleQuery(dbmock, role)

	signedEditRoute := server.SignedRoleRoute(role, conf)
	respBody, resp := sendRequest(t, signedEditRoute, nil)

	assert.Equal(t, 200, resp.StatusCode)

	assert.Regexp(t, fmt.Sprintf(`<input.+name="name".*value="%s".*>`, role.Name), respBody)
	assert.Regexp(t, fmt.Sprintf(`<input.+name="role".*value="%s".*>`, role.Role), respBody)
	assert.Regexp(t, fmt.Sprintf(`<textarea.+name="resume".*>%s</textarea>`, role.Resume), respBody)
}

func TestUpdateJobAuthorized(t *testing.T) {
	s, svcmock, dbmock, conf := makeServer(t)
	defer s.Close()

	tests := []struct {
		values            map[string][]string
		expectSuccess     bool
		expectErrMessages []string
	}{
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {""},
				"url":          {"https://devict.org"},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {"Super rad place to work"},
				"url":          {""},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {""},
				"url":          {""},
			},
			expectSuccess:     false,
			expectErrMessages: []string{data.ErrNoUrlOrDescription},
		},
		{
			values: map[string][]string{
				"position":     {"Pos"},
				"organization": {"Org"},
				"description":  {""},
				"url":          {"invalid"},
			},
			expectSuccess:     false,
			expectErrMessages: []string{data.ErrInvalidUrl},
		},
	}

	for _, tt := range tests {
		job := data.Job{
			ID:           "1",
			Position:     "Original Position",
			Organization: "Original Organization",
			Url:          sql.NullString{Valid: false},
			Description:  sql.NullString{String: "Original Description", Valid: true},
			Email:        "secret@secret.com",
			PublishedAt:  time.Now(),
		}

		// Expect requireAuth query
		expectGetJobQuery(dbmock, job)

		desc := tt.values["description"][0]
		urlVal := tt.values["url"][0]
		newParams := data.Job{
			ID:           job.ID,
			Position:     tt.values["position"][0],
			Organization: tt.values["organization"][0],
			Description:  sql.NullString{String: desc, Valid: desc != ""},
			Url:          sql.NullString{String: urlVal, Valid: urlVal != ""},
			PublishedAt:  job.PublishedAt,
			Email:        job.Email,
		}

		if tt.expectSuccess {
			expectGetJobQuery(dbmock, job)

			dbmock.ExpectExec(`UPDATE jobs .+ WHERE id = .+`).WithArgs(
				tt.values["position"][0],
				tt.values["organization"][0],
				sql.NullString{String: urlVal, Valid: urlVal != ""},
				sql.NullString{String: desc, Valid: desc != ""},
				job.ID,
			).WillReturnResult(sqlmock.NewResult(0, 1))

			expectSelectJobsQuery(dbmock, []data.Job{newParams})
			expectSelectRolesQuery(dbmock, []data.Role{})
		} else {
			// On failure, expect twice again for the redirect to /edit
			// which calls requireAuth, and then getJob for the view
			expectGetJobQuery(dbmock, job)
			expectGetJobQuery(dbmock, job)
		}

		reqBody := url.Values(tt.values).Encode()
		route := fmt.Sprintf(
			"%s/jobs/%s?token=%s",
			s.URL,
			job.ID,
			job.AuthSignature(conf.AppSecret),
		)
		respBody, resp := sendRequest(t, route, []byte(reqBody))

		// Should follow the redirect and result in a 200 regardless of success/failure
		assert.Equal(t, 200, resp.StatusCode)

		// Should not resend any notifications on updates
		assert.Empty(t, svcmock.emails)
		assert.Empty(t, svcmock.tweets)
		assert.Empty(t, svcmock.jobSlacks)
		assert.Empty(t, svcmock.roleSlacks)

		if tt.expectSuccess {
			assert.Contains(t, respBody, tt.values["position"][0])
			assert.Contains(t, respBody, tt.values["organization"][0])
		} else {
			for _, errMsg := range tt.expectErrMessages {
				assert.Contains(t, respBody, errMsg)
			}
		}
	}
}

func TestUpdateRoleAuthorized(t *testing.T) {
	s, svcmock, dbmock, conf := makeServer(t)
	defer s.Close()

	tests := []struct {
		values            map[string][]string
		expectSuccess     bool
		expectErrMessages []string
	}{
		{
			values: map[string][]string{
				"name":     {"Test McTestington"},
				"email":    {"test@example.com"},
				"phone":    {"316-555-1111"},
				"role":     {"role 1"},
				"resume":   {"# Cool\n\nwow"},
				"linkedin": {"https://www.linkedin.com/example"},
				"website":  {""},
				"github":   {""},
				"complow":  {""},
				"comphigh": {""},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"name":     {"Test McTestington"},
				"email":    {"test@example.com"},
				"phone":    {""},
				"role":     {"role 2"},
				"resume":   {"# Cool\n\nwow"},
				"linkedin": {""},
				"website":  {""},
				"github":   {""},
				"complow":  {""},
				"comphigh": {""},
			},
			expectSuccess: true,
		},
		{
			values: map[string][]string{
				"name":     {"Test McTestington"},
				"email":    {"test@example.com"},
				"phone":    {""},
				"role":     {""},
				"resume":   {"# Cool\n\nwow"},
				"linkedin": {""},
				"website":  {""},
				"github":   {""},
				"complow":  {""},
				"comphigh": {""},
			},
			expectSuccess:     false,
			expectErrMessages: []string{data.ErrNoRole},
		},
	}

	for _, tt := range tests {
		role := data.Role{
			ID:          "1",
			Name:        "Original Name",
			Email:       "test@example.com",
			Phone:       sql.NullString{Valid: false},
			Role:        "Original Role",
			Resume:      "Original Resume",
			Linkedin:    sql.NullString{Valid: false},
			Website:     sql.NullString{Valid: false},
			Github:      sql.NullString{Valid: false},
			CompLow:     sql.NullString{Valid: false},
			CompHigh:    sql.NullString{Valid: false},
			PublishedAt: time.Now(),
		}

		// Expect requireAuth query
		expectGetRoleQuery(dbmock, role)

		newParams := data.Role{
			ID:          role.ID,
			Name:        tt.values["name"][0],
			Email:       role.Email,
			Phone:       sql.NullString{String: tt.values["phone"][0], Valid: tt.values["phone"][0] != ""},
			Role:        tt.values["role"][0],
			Resume:      tt.values["resume"][0],
			Linkedin:    sql.NullString{String: tt.values["linkedin"][0], Valid: tt.values["linkedin"][0] != ""},
			Website:     sql.NullString{String: tt.values["website"][0], Valid: tt.values["website"][0] != ""},
			Github:      sql.NullString{String: tt.values["github"][0], Valid: tt.values["github"][0] != ""},
			CompLow:     sql.NullString{String: tt.values["complow"][0], Valid: tt.values["complow"][0] != ""},
			CompHigh:    sql.NullString{String: tt.values["comphigh"][0], Valid: tt.values["comphigh"][0] != ""},
			PublishedAt: role.PublishedAt,
		}

		if tt.expectSuccess {
			expectGetRoleQuery(dbmock, role)

			dbmock.ExpectExec(`UPDATE roles .+ WHERE id = .+`).WithArgs(
				tt.values["name"][0],
				sql.NullString{String: tt.values["phone"][0], Valid: tt.values["phone"][0] != ""},
				tt.values["role"][0],
				tt.values["resume"][0],
				sql.NullString{String: tt.values["linkedin"][0], Valid: tt.values["linkedin"][0] != ""},
				sql.NullString{String: tt.values["website"][0], Valid: tt.values["website"][0] != ""},
				sql.NullString{String: tt.values["github"][0], Valid: tt.values["github"][0] != ""},
				sql.NullString{String: tt.values["complow"][0], Valid: tt.values["complow"][0] != ""},
				sql.NullString{String: tt.values["comphigh"][0], Valid: tt.values["comphigh"][0] != ""},
				role.ID,
			).WillReturnResult(sqlmock.NewResult(0, 1))

			expectSelectJobsQuery(dbmock, []data.Job{})
			expectSelectRolesQuery(dbmock, []data.Role{newParams})
		} else {
			// On failure, expect twice again for the redirect to /edit
			// which calls requireAuth, and then getJob for the view
			expectGetRoleQuery(dbmock, role)
			expectGetRoleQuery(dbmock, role)
		}

		reqBody := url.Values(tt.values).Encode()
		route := fmt.Sprintf(
			"%s/roles/%s?token=%s",
			s.URL,
			role.ID,
			role.AuthSignature(conf.AppSecret),
		)
		respBody, resp := sendRequest(t, route, []byte(reqBody))

		// Should follow the redirect and result in a 200 regardless of success/failure
		assert.Equal(t, 200, resp.StatusCode)

		// Should not resend any notifications on updates
		assert.Empty(t, svcmock.emails)
		assert.Empty(t, svcmock.tweets)
		assert.Empty(t, svcmock.jobSlacks)
		assert.Empty(t, svcmock.roleSlacks)

		if tt.expectSuccess {
			assert.Contains(t, respBody, tt.values["name"][0])
			assert.Contains(t, respBody, tt.values["role"][0])
		} else {
			for _, errMsg := range tt.expectErrMessages {
				assert.Contains(t, respBody, errMsg)
			}
		}
	}
}

// Helpers ------------------------------

type email struct {
	recipient, subject, body string
}

type mockService struct {
	emails     []email
	tweets     []data.Job
	jobSlacks  []data.Job
	roleSlacks []data.Role
}

func (svc *mockService) SendEmail(recipient, subject, body string) error {
	svc.emails = append(svc.emails, email{recipient, subject, body})
	return nil
}

func (svc *mockService) PostToTwitter(job data.Job) error {
	svc.tweets = append(svc.tweets, job)
	return nil
}

func (svc *mockService) PostJobToSlack(job data.Job) error {
	svc.jobSlacks = append(svc.jobSlacks, job)
	return nil
}

func (svc *mockService) PostRoleToSlack(role data.Role) error {
	svc.roleSlacks = append(svc.roleSlacks, role)
	return nil
}

func makeServer(t *testing.T) (*httptest.Server, *mockService, sqlmock.Sqlmock, *config.Config) {
	db, dbmock, err := sqlmock.New()
	assert.NoError(t, err)

	conf := &config.Config{AppSecret: "sup", Env: "debug"}
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

	// We need a cookie jar so cookies are retained between redirects
	cookieJar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	assert.NoError(t, err)

	client := http.Client{Jar: cookieJar}

	if postBody == nil {
		resp, err = client.Get(path)
	} else {
		// TODO: switch this to client.PostForm to simplify
		resp, err = client.Post(path, "application/x-www-form-urlencoded", bytes.NewReader(postBody))
	}

	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	time.Sleep(1 * time.Millisecond) // Makes sure the resp object is populated properly
	return string(body), resp
}

func resetServiceMock(svc *mockService) {
	svc.emails = []email{}
	svc.tweets = []data.Job{}
	svc.jobSlacks = []data.Job{}
	svc.roleSlacks = []data.Role{}
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

func mockRoleRow(role data.Role) []driver.Value {
	vals := []driver.Value{
		"1",
		"Test Testington",
		"example@example.com",
		sql.NullString{},
		"All roles",
		"wow cool",
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		sql.NullString{},
		time.Now(),
	}

	if role.ID != "" {
		vals[0] = role.ID
	}

	if role.Name != "" {
		vals[1] = role.Name
	}

	if role.Email != "" {
		vals[2] = role.Email
	}

	if role.Phone.Valid {
		vals[3] = role.Phone
	}

	if role.Role != "" {
		vals[4] = role.Role
	}

	if role.Resume != "" {
		vals[5] = role.Resume
	}

	if role.Linkedin.Valid {
		vals[6] = role.Linkedin
	}

	if role.Website.Valid {
		vals[7] = role.Website
	}

	if role.Github.Valid {
		vals[8] = role.Github
	}

	if role.CompLow.Valid {
		vals[9] = role.CompLow
	}

	if role.CompHigh.Valid {
		vals[10] = role.CompHigh
	}

	if !role.PublishedAt.IsZero() {
		vals[11] = role.PublishedAt
	}

	return vals
}

func expectSelectJobsQuery(dbmock sqlmock.Sqlmock, jobs []data.Job) {
	rows := sqlmock.NewRows(getDbFields(data.Job{}))
	for _, job := range jobs {
		rows.AddRow(mockJobRow(job)...)
	}
	dbmock.ExpectQuery(`SELECT \* FROM jobs`).WillReturnRows(rows)
}

func expectSelectRolesQuery(dbmock sqlmock.Sqlmock, roles []data.Role) {
	rows := sqlmock.NewRows(getDbFields(data.Role{}))
	for _, role := range roles {
		rows.AddRow(mockRoleRow(role)...)
	}
	dbmock.ExpectQuery(`SELECT \* FROM roles`).WillReturnRows(rows)
}

// TODO: use this everywhere
func expectGetJobQuery(dbmock sqlmock.Sqlmock, job data.Job) {
	dbmock.ExpectQuery(`SELECT \* FROM jobs.+`).WillReturnRows(
		sqlmock.NewRows(getDbFields(data.Job{})).AddRow(mockJobRow(job)...),
	)
}

func expectGetRoleQuery(dbmock sqlmock.Sqlmock, role data.Role) {
	dbmock.ExpectQuery(`SELECT \* FROM roles.+`).WillReturnRows(
		sqlmock.NewRows(getDbFields(data.Role{})).AddRow(mockRoleRow(role)...),
	)
}
