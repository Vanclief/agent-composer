package triggers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/vanclief/ez"
)

const (
	// DefaultGitHubAPI is the GitHub REST API; tests point the poller at
	// an httptest server instead.
	DefaultGitHubAPI = "https://api.github.com"

	perPage  = 100
	maxPages = 10
	// pullsPages bounds how far back the reviews source walks the pull
	// requests sorted by last update.
	pullsPages = 3
)

// The GitHub source's kinds, one cursor each per repo.
const (
	KindIssueComments  = "issue_comments"
	KindReviewComments = "review_comments"
	KindReviews        = "reviews"
	KindIssues         = "issues"
	KindRuns           = "runs"
)

// GitHubKinds lists the kinds in the order a poll walks them.
var GitHubKinds = []string{KindIssues, KindIssueComments, KindReviewComments, KindReviews, KindRuns}

// githubClient makes authenticated GET requests to the GitHub API. Its
// errors name the path and status, never the token.
type githubClient struct {
	base  string
	token string
	http  *http.Client
}

func (c *githubClient) get(ctx context.Context, apiPath string, query url.Values, out interface{}) error {
	target := strings.TrimRight(c.base, "/") + apiPath
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return ez.Wrap(err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "agc-triggers")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ez.New(ez.EINTERNAL, "github GET "+apiPath+" failed", nil)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return ez.New(ez.EINTERNAL, "github GET "+apiPath+": cannot read the response", nil)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("github GET %s: %s", apiPath, resp.Status)
		var gh struct {
			Message string `json:"message"`
		}
		jsonErr := json.Unmarshal(body, &gh)
		if jsonErr == nil && gh.Message != "" {
			msg += ": " + gh.Message
		}
		return ez.New(ez.EINTERNAL, msg, nil)
	}

	err = json.Unmarshal(body, out)
	if err != nil {
		return ez.New(ez.EINTERNAL, "github GET "+apiPath+": the response is not the expected JSON", err)
	}

	return nil
}

// GitHub API shapes, only the fields agc reads.

type ghUser struct {
	Login string `json:"login"`
}

type ghComment struct {
	ID             int64     `json:"id"`
	User           ghUser    `json:"user"`
	Body           string    `json:"body"`
	HTMLURL        string    `json:"html_url"`
	IssueURL       string    `json:"issue_url"`
	PullRequestURL string    `json:"pull_request_url"`
	Path           string    `json:"path"`
	CreatedAt      time.Time `json:"created_at"`
}

type ghIssue struct {
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	User        ghUser     `json:"user"`
	HTMLURL     string     `json:"html_url"`
	CreatedAt   time.Time  `json:"created_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	ClosedBy    *ghUser    `json:"closed_by"`
	PullRequest *struct {
		MergedAt *time.Time `json:"merged_at"`
	} `json:"pull_request"`
}

type ghPull struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	HTMLURL   string     `json:"html_url"`
	UpdatedAt time.Time  `json:"updated_at"`
	MergedAt  *time.Time `json:"merged_at"`
	MergedBy  *ghUser    `json:"merged_by"`
	Head      struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

type ghReview struct {
	ID          int64     `json:"id"`
	User        ghUser    `json:"user"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type ghRun struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	DisplayTitle    string    `json:"display_title"`
	HeadBranch      string    `json:"head_branch"`
	Status          string    `json:"status"`
	Conclusion      string    `json:"conclusion"`
	RunAttempt      int       `json:"run_attempt"`
	HTMLURL         string    `json:"html_url"`
	UpdatedAt       time.Time `json:"updated_at"`
	TriggeringActor *ghUser   `json:"triggering_actor"`
	Actor           *ghUser   `json:"actor"`
	PullRequests    []struct {
		Number int `json:"number"`
	} `json:"pull_requests"`
}

type ghRuns struct {
	WorkflowRuns []ghRun `json:"workflow_runs"`
}

// repoPoll is one poll of one repo: the client, the window start and the
// pull requests fetched so far, so a PR is read once per poll.
type repoPoll struct {
	client *githubClient
	repo   string
	since  time.Time
	pulls  map[int]*ghPull
}

func (p *repoPoll) repoPath(rest string) string {
	return "/repos/" + p.repo + rest
}

// pull returns pull request n, from the cache when this poll read it.
func (p *repoPoll) pull(ctx context.Context, n int) (*ghPull, error) {
	cached, ok := p.pulls[n]
	if ok {
		return cached, nil
	}

	pr := new(ghPull)
	err := p.client.get(ctx, p.repoPath("/pulls/"+strconv.Itoa(n)), nil, pr)
	if err != nil {
		return nil, err
	}

	p.pulls[n] = pr
	return pr, nil
}

// pages fetches up to maxPages pages of a list endpoint into a slice
// each, stopping at the first short page.
func pages[T any](ctx context.Context, p *repoPoll, apiPath string, query url.Values) ([]T, error) {
	all := []T{}
	for page := 1; page <= maxPages; page++ {
		q := url.Values{}
		for key, values := range query {
			q[key] = values
		}
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))

		batch := []T{}
		err := p.client.get(ctx, apiPath, q, &batch)
		if err != nil {
			return nil, err
		}

		all = append(all, batch...)
		if len(batch) < perPage {
			break
		}
	}

	return all, nil
}

func sinceParam(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// numberFromURL reads the trailing number of an API or HTML URL such as
// .../issues/12 or .../pull/12#issuecomment-1.
func numberFromURL(raw string) int {
	trimmed, _, _ := strings.Cut(raw, "#")
	n, err := strconv.Atoi(path.Base(trimmed))
	if err != nil {
		return 0
	}

	return n
}

// prPlace names a pull request with its head branch: PR #12 (agc/x).
func prPlace(pr *ghPull) string {
	return fmt.Sprintf("PR #%d (%s)", pr.Number, pr.Head.Ref)
}

func (p *repoPoll) issueComments(ctx context.Context) ([]Event, error) {
	query := url.Values{"since": {sinceParam(p.since)}, "sort": {"updated"}, "direction": {"asc"}}
	comments, err := pages[ghComment](ctx, p, p.repoPath("/issues/comments"), query)
	if err != nil {
		return nil, err
	}

	events := []Event{}
	for _, comment := range comments {
		if comment.CreatedAt.Before(p.since) {
			continue
		}

		n := numberFromURL(comment.IssueURL)
		event := Event{
			Key:   fmt.Sprintf("comment:%d", comment.ID),
			At:    comment.CreatedAt,
			Repo:  p.repo,
			Actor: comment.User.Login,
			Text:  comment.Body,
			URL:   comment.HTMLURL,
		}

		if strings.Contains(comment.HTMLURL, "/pull/") {
			pr, err := p.pull(ctx, n)
			if err != nil {
				return nil, err
			}
			event.Branch = pr.Head.Ref
			event.What = "commented on " + prPlace(pr)
		} else {
			event.What = fmt.Sprintf("commented on issue #%d", n)
		}

		events = append(events, event)
	}

	return events, nil
}

func (p *repoPoll) reviewComments(ctx context.Context) ([]Event, error) {
	query := url.Values{"since": {sinceParam(p.since)}, "sort": {"updated"}, "direction": {"asc"}}
	comments, err := pages[ghComment](ctx, p, p.repoPath("/pulls/comments"), query)
	if err != nil {
		return nil, err
	}

	events := []Event{}
	for _, comment := range comments {
		if comment.CreatedAt.Before(p.since) {
			continue
		}

		pr, err := p.pull(ctx, numberFromURL(comment.PullRequestURL))
		if err != nil {
			return nil, err
		}

		what := "commented on " + prPlace(pr)
		if comment.Path != "" {
			what = "commented on " + comment.Path + " in " + prPlace(pr)
		}

		events = append(events, Event{
			Key:    fmt.Sprintf("review_comment:%d", comment.ID),
			At:     comment.CreatedAt,
			Repo:   p.repo,
			Actor:  comment.User.Login,
			What:   what,
			Text:   comment.Body,
			URL:    comment.HTMLURL,
			Branch: pr.Head.Ref,
		})
	}

	return events, nil
}

// reviews walks the pull requests by last update, newest first, until one
// is older than the window, and reads the reviews of each. A COMMENTED
// review without a body is the wrapper of single review comments, which
// the review_comments kind already posts.
func (p *repoPoll) reviews(ctx context.Context) ([]Event, error) {
	recent := []ghPull{}
	for page := 1; page <= pullsPages; page++ {
		query := url.Values{
			"state":     {"all"},
			"sort":      {"updated"},
			"direction": {"desc"},
			"per_page":  {strconv.Itoa(perPage)},
			"page":      {strconv.Itoa(page)},
		}

		batch := []ghPull{}
		err := p.client.get(ctx, p.repoPath("/pulls"), query, &batch)
		if err != nil {
			return nil, err
		}

		done := len(batch) < perPage
		for _, pr := range batch {
			if pr.UpdatedAt.Before(p.since) {
				done = true
				break
			}
			recent = append(recent, pr)
		}

		if done {
			break
		}
	}

	events := []Event{}
	for i := range recent {
		pr := &recent[i]
		p.pulls[pr.Number] = pr

		reviews, err := pages[ghReview](ctx, p, p.repoPath(fmt.Sprintf("/pulls/%d/reviews", pr.Number)), nil)
		if err != nil {
			return nil, err
		}

		for _, review := range reviews {
			if review.SubmittedAt.Before(p.since) {
				continue
			}

			verb := ""
			switch review.State {
			case "APPROVED":
				verb = "approved "
			case "CHANGES_REQUESTED":
				verb = "requested changes on "
			case "COMMENTED":
				if strings.TrimSpace(review.Body) == "" {
					continue
				}
				verb = "reviewed "
			default:
				continue
			}

			events = append(events, Event{
				Key:    fmt.Sprintf("review:%d", review.ID),
				At:     review.SubmittedAt,
				Repo:   p.repo,
				Actor:  review.User.Login,
				What:   verb + prPlace(pr),
				Text:   review.Body,
				URL:    review.HTMLURL,
				Branch: pr.Head.Ref,
			})
		}
	}

	return events, nil
}

// issues posts issues and pull requests opened, and closed or merged,
// inside the window.
func (p *repoPoll) issues(ctx context.Context) ([]Event, error) {
	query := url.Values{"state": {"all"}, "since": {sinceParam(p.since)}, "sort": {"updated"}, "direction": {"asc"}}
	issues, err := pages[ghIssue](ctx, p, p.repoPath("/issues"), query)
	if err != nil {
		return nil, err
	}

	events := []Event{}
	for _, issue := range issues {
		opened := !issue.CreatedAt.Before(p.since)
		closed := issue.ClosedAt != nil && !issue.ClosedAt.Before(p.since)
		if !opened && !closed {
			continue
		}

		place := fmt.Sprintf("issue #%d", issue.Number)
		branch := ""
		var pr *ghPull
		if issue.PullRequest != nil {
			pr, err = p.pull(ctx, issue.Number)
			if err != nil {
				return nil, err
			}
			place = prPlace(pr)
			branch = pr.Head.Ref
		}

		text := issue.Title
		if strings.TrimSpace(issue.Body) != "" {
			text = issue.Title + ": " + issue.Body
		}

		if opened {
			events = append(events, Event{
				Key:    fmt.Sprintf("opened:%d", issue.Number),
				At:     issue.CreatedAt,
				Repo:   p.repo,
				Actor:  issue.User.Login,
				What:   "opened " + place,
				Text:   text,
				URL:    issue.HTMLURL,
				Branch: branch,
			})
		}

		if closed {
			event := Event{
				Key:    fmt.Sprintf("closed:%d", issue.Number),
				At:     *issue.ClosedAt,
				Repo:   p.repo,
				What:   "closed " + place,
				Text:   issue.Title,
				URL:    issue.HTMLURL,
				Branch: branch,
			}
			if issue.ClosedBy != nil {
				event.Actor = issue.ClosedBy.Login
			}
			if pr != nil && pr.MergedAt != nil {
				event.Key = fmt.Sprintf("merged:%d", issue.Number)
				event.What = "merged " + place
				if pr.MergedBy != nil {
					event.Actor = pr.MergedBy.Login
				}
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// runs posts workflow runs that finished inside the window: the first
// page of completed runs, newest first, is enough at a poll of a minute.
func (p *repoPoll) runs(ctx context.Context) ([]Event, error) {
	query := url.Values{"status": {"completed"}, "per_page": {strconv.Itoa(perPage)}}
	list := new(ghRuns)
	err := p.client.get(ctx, p.repoPath("/actions/runs"), query, list)
	if err != nil {
		return nil, err
	}

	events := []Event{}
	for _, run := range list.WorkflowRuns {
		if run.Status != "completed" || run.UpdatedAt.Before(p.since) {
			continue
		}

		where := "on " + run.HeadBranch
		if len(run.PullRequests) > 0 {
			where = fmt.Sprintf("on PR #%d (%s)", run.PullRequests[0].Number, run.HeadBranch)
		}

		actor := ""
		if run.TriggeringActor != nil {
			actor = run.TriggeringActor.Login
		} else if run.Actor != nil {
			actor = run.Actor.Login
		}

		conclusion := run.Conclusion
		if conclusion == "" {
			conclusion = "done"
		}

		events = append(events, Event{
			Key:    fmt.Sprintf("run:%d:%d", run.ID, run.RunAttempt),
			At:     run.UpdatedAt,
			Repo:   p.repo,
			Actor:  actor,
			What:   fmt.Sprintf("ran workflow %q %s: %s", run.Name, where, conclusion),
			Text:   run.DisplayTitle,
			URL:    run.HTMLURL,
			Branch: run.HeadBranch,
		})
	}

	return events, nil
}

// fetch runs one kind.
func (p *repoPoll) fetch(ctx context.Context, kind string) ([]Event, error) {
	switch kind {
	case KindIssueComments:
		return p.issueComments(ctx)
	case KindReviewComments:
		return p.reviewComments(ctx)
	case KindReviews:
		return p.reviews(ctx)
	case KindIssues:
		return p.issues(ctx)
	case KindRuns:
		return p.runs(ctx)
	}

	return nil, ez.New(ez.EINVALID, "unknown github kind "+kind, nil)
}
