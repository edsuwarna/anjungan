package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/edsuwarna/anjungan/internal/common/model"
)

// GitHubAdapter implements GitProvider for github.com.
type GitHubAdapter struct {
	token        string
	affiliations string
}

const githubAPIBase = "https://api.github.com"

type ghRepo struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Language    string `json:"language"`
	Visibility  string `json:"visibility"`
	CloneURL    string `json:"clone_url"`
	HTMLURL     string `json:"html_url"`
	UpdatedAt   string `json:"updated_at"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
	Name string `json:"name"`
}

type ghCommitStatus struct {
	State string `json:"state"`
}

type ghPR struct {
	Number int `json:"number"`
}

func (g *GitHubAdapter) Name() string { return "github" }

func (g *GitHubAdapter) Validate(ctx context.Context) (*ProviderIdentity, error) {
	resp, err := g.doRequest("/user")
	if err != nil {
		return nil, fmt.Errorf("github validate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github validate: invalid token (HTTP %d)", resp.StatusCode)
	}

	var user struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("github validate decode: %w", err)
	}

	return &ProviderIdentity{Login: user.Login, Name: user.Name}, nil
}

func (g *GitHubAdapter) doRequest(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", githubAPIBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

func (g *GitHubAdapter) ListRepos(ctx context.Context) ([]model.GitRepo, error) {
	// Use affiliation filter if non-default, otherwise fall back to type=all
	var path string
	if g.affiliations != "" && g.affiliations != "owner,collaborator,organization_member" {
		path = "/user/repos?per_page=100&sort=updated&affiliation=" + url.QueryEscape(g.affiliations)
	} else {
		path = "/user/repos?per_page=100&sort=updated&type=all"
	}
	resp, err := g.doRequest(path)
	if err != nil {
		return nil, fmt.Errorf("github list repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API error: %s", resp.Status)
	}

	var repos []ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("github decode: %w", err)
	}

	var result []model.GitRepo
	for _, r := range repos {
		result = append(result, model.GitRepo{
			Provider:      "github",
			Owner:         r.Owner.Login,
			Name:          r.Name,
			FullName:      r.FullName,
			Description:   r.Description,
			DefaultBranch: r.DefaultBranch,
			Language:      r.Language,
			Visibility:    r.Visibility,
			CloneURL:      r.CloneURL,
			HTMLURL:       r.HTMLURL,
			UpdatedAt:     r.UpdatedAt,
		})
	}
	return result, nil
}

func (g *GitHubAdapter) ListBranches(ctx context.Context, owner, repo string) ([]string, error) {
	resp, err := g.doRequest(fmt.Sprintf("/repos/%s/%s/branches?per_page=100", owner, repo))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}
	var names []string
	for _, b := range branches {
		names = append(names, b.Name)
	}
	return names, nil
}

func (g *GitHubAdapter) GetCommitStatus(ctx context.Context, owner, repo, branch string) (*model.RepoCIStatus, error) {
	if branch == "" {
		branch = "main"
	}
	resp, err := g.doRequest(fmt.Sprintf("/repos/%s/%s/commits/%s/status", owner, repo, branch))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If branch doesn't exist yet, return nil gracefully
		return nil, nil
	}

	var status ghCommitStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &model.RepoCIStatus{
		Provider: "github",
		Owner:    owner,
		Repo:     repo,
		Branch:   branch,
		State:    status.State,
	}, nil
}

func (g *GitHubAdapter) ListOpenPRs(ctx context.Context, owner, repo string) (int, error) {
	resp, err := g.doRequest(fmt.Sprintf("/repos/%s/%s/pulls?state=open&per_page=1", owner, repo))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Parse Link header or just count the array
	var prs []ghPR
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return 0, err
	}
	return len(prs), nil
}
