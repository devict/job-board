package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
)

type ISlackService interface {
	PostToSlack(data.Job) error
}

type SlackService struct {
	Conf *config.Config
}

type SlackMessage struct {
	Text string `json:"text"`
}

func (svc *SlackService) PostToSlack(job data.Job) error {
	message := slackMessageFromJob(job, svc.Conf)
	messageStr, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	_, err = http.Post(svc.Conf.SlackHook, "application/json", bytes.NewReader(messageStr))
	if err != nil {
		return fmt.Errorf("failed to post to slack: %w", err)
	}

	return nil
}

func slackMessageFromJob(job data.Job, c *config.Config) SlackMessage {
	text := fmt.Sprintf(
		"A new job was posted!\n> *<%s/jobs/%s|%s @ %s>*",
		c.URL,
		job.ID,
		job.Position,
		job.Organization,
	)
	return SlackMessage{Text: text}
}
