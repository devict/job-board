package server

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
)

func SignatureForJob(job data.Job, secret string) string {
	input := fmt.Sprintf(
		"%s:%s:%s:%s",
		job.ID,
		job.Email,
		job.PublishedAt.String(),
		secret,
	)

	hash := sha1.New()
	hash.Write([]byte(input))

	return string(base64.URLEncoding.EncodeToString(hash.Sum(nil)))
}

func SignedJobRoute(job data.Job, c *config.Config) string {
	return fmt.Sprintf(
		"%s/jobs/%s/edit?token=%s",
		c.URL,
		job.ID,
		url.QueryEscape(SignatureForJob(job, c.AppSecret)),
	)
}
