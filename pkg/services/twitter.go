package services

import (
	"fmt"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
	"github.com/dghubble/go-twitter/twitter"
	"github.com/dghubble/oauth1"
)

type ITwitterService interface {
	PostToTwitter(data.Job) error
}

type TwitterService struct {
	Conf *config.Config
}

func (svc *TwitterService) PostToTwitter(job data.Job) error {
	tweetStr := tweetFromJob(job, svc.Conf)

	oa := oauth1.NewConfig(svc.Conf.Twitter.APIKey, svc.Conf.Twitter.APISecretKey)
	token := oauth1.NewToken(
		svc.Conf.Twitter.AccessToken,
		svc.Conf.Twitter.AccessTokenSecret,
	)
	httpClient := oa.Client(oauth1.NoContext, token)

	twClient := twitter.NewClient(httpClient)

	// TODO: check for failures in the resp object?
	_, _, err := twClient.Statuses.Update(tweetStr, nil)
	if err != nil {
		return fmt.Errorf("failed to post to twitter: %w", err)
	}

	return nil
}

func tweetFromJob(job data.Job, c *config.Config) string {
	return fmt.Sprintf(
		"A job was posted! -- %s at %s\n\nMore info at %s/jobs/%s",
		job.Position,
		job.Organization,
		c.URL,
		job.ID,
	)
}
