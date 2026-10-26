// Package inbox lists PRs awaiting the user's review, via the gh CLI.
package inbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/breyr/brb/runner"
)

// PR is one pull request awaiting review. Comments counts what gh reports as
// commentsCount.
type PR struct {
	Repo     string // owner/name
	Number   int
	Title    string
	Author   string
	URL      string
	Comments int
}

// Review returns open PRs with a review requested from `git config brb.user`,
// else the account gh is signed in as. shortcut: gh's default page of 30; add
// paging if an inbox ever outgrows it.
func Review(r runner.Runner) ([]PR, error) {
	login, err := r.Run("git", "config", "brb.user")
	if err != nil || login == "" {
		login = "@me"
	}

	out, err := r.Run("gh", "search", "prs", "--review-requested="+login, "--state=open",
		"--json", "number,title,url,repository,author,commentsCount")
	if errors.Is(err, exec.ErrNotFound) {
		return nil, errors.New("gh is not installed")
	}

	if err != nil {
		return nil, fmt.Errorf("gh search prs: %w", err)
	}

	var raw []struct {
		Number        int    `json:"number"`
		Title         string `json:"title"`
		URL           string `json:"url"`
		CommentsCount int    `json:"commentsCount"`
		Repository    struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
	}

	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}

	prs := make([]PR, len(raw))
	for i, p := range raw {
		prs[i] = PR{p.Repository.NameWithOwner, p.Number, p.Title, p.Author.Login, p.URL, p.CommentsCount}
	}

	return prs, nil
}
