package server

import (
	"fmt"
	"net/url"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
)

func SignedJobRoute(job data.Job, c *config.Config) string {
	return fmt.Sprintf(
		"%s/jobs/%s/edit?token=%s",
		c.URL,
		job.ID,
		url.QueryEscape(job.AuthSignature(c.AppSecret)),
	)
}

func SignedRoleRoute(role data.Role, c *config.Config) string {
	return fmt.Sprintf(
		"%s/roles/%s/edit?token=%s",
		c.URL,
		role.ID,
		url.QueryEscape(role.AuthSignature(c.AppSecret)),
	)
}
