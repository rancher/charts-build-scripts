package utils

import (
	"context"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// getGithubClient creates a client that can make request to the Github API
func getGithubClient(ctx context.Context, githubToken string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: githubToken,
	})
	return github.NewClient(oauth2.NewClient(ctx, ts))
}
