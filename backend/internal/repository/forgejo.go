package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/edsuwarna/anjungan/internal/common/model"
)

// ForgejoAdapter implements GitProvider for self-hosted Forgejo/Gitea instances.
type ForgejoAdapter struct {
	token        string
	baseURL      string
	affiliations string
}

type fjUser struct {
	Login string `json:"login"`
}

type fjRepo struct {
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	Language      string `json:"language"`
	Visibility    string `json:"visibility"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
	UpdatedAt     string `json:"updated_at"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
	Name string `json:"name"`
}

type fjCommitStatus struct {
	State string `json:"state"`
}

type fjPR struct {
	Number int `json:"number"`
}

func (f *ForgejoAdapter) Name() string { return "forgejo" }

func (f *ForgejoAdapter) Validate(ctx context.Context) (*ProviderIdentity, error) {
	resp, err := f.doRequest("/user")
	if err != nil {
		return nil, fmt.Errorf("forgejo validate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forgejo validate: invalid token (HTTP %d)", resp.StatusCode)
	}

	var user struct {
		Login string `json:"login"`
		FullName string `json:"full_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("forgejo validate decode: %w", err)
	}

	return &ProviderIdentity{Login: user.Login, Name: user.FullName}, nil
}

func (f *ForgejoAdapter) apiURL() string {
	base := strings.TrimRight(f.baseURL, "/")
	return base + "/api/v1"
}

func (f *ForgejoAdapter) doRequest(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", f.apiURL()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+f.token)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

func (f *ForgejoAdapter) ListRepos(ctx context.Context) ([]model.GitRepo, error) {
	resp, err := f.doRequest("/user/repos?limit=100&sort=updated")
	if err != nil {
		return nil, fmt.Errorf("forgejo list repos: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("forgejo API error: %s", resp.Status)
	}

	var repos []fjRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("forgejo decode: %w", err)
	}

	// Determine the current user's login for affiliation filtering
	var currentUserLogin string
	applyFilter := f.affiliations != "" && f.affiliations != "owner,collaborator,organization_member"
	if applyFilter {
		userResp, err := f.doRequest("/user")
		if err == nil {
			defer userResp.Body.Close()
			var u fjUser
			if json.NewDecoder(userResp.Body).Decode(&u) == nil {
				currentUserLogin = u.Login
			}
		}
	}

	wanted := strings.Split(f.affiliations, ",")
	var result []model.GitRepo
	for _, r := range repos {
		// Filter by affiliation if needed
		if applyFilter && currentUserLogin != "" {
			isOwner := strings.EqualFold(r.Owner.Login, currentUserLogin)
			wantsOwner := containsAffiliation(wanted, "owner")
			wantsNonOwner := containsAffiliation(wanted, "collaborator") || containsAffiliation(wanted, "organization_member")
			if isOwner && !wantsOwner {
				continue
			}
			if !isOwner && !wantsNonOwner {
				continue
			}
		}
		result = append(result, model.GitRepo{
			Provider:      "forgejo",
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

func containsAffiliation(list []string, target string) bool {
	for _, a := range list {
		if strings.TrimSpace(a) == target {
			return true
		}
	}
	return false
}

func (f *ForgejoAdapter) ListBranches(ctx context.Context, owner, repo string) ([]string, error) {
	resp, err := f.doRequest(fmt.Sprintf("/repos/%s/%s/branches?limit=100", owner, repo))
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

func (f *ForgejoAdapter) GetCommitStatus(ctx context.Context, owner, repo, branch string) (*model.RepoCIStatus, error) {
	if branch == "" {
		branch = "main"
	}
	// Forgejo/Gitea doesn't have a combined commit status endpoint like GitHub.
	// Try to get the latest commit status via the repository's branch.
	resp, err := f.doRequest(fmt.Sprintf("/repos/%s/%s/branches/%s", owner, repo, branch))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var branchData struct {
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branchData); err != nil {
		return nil, err
	}

	// Get statuses for the latest commit
	statusResp, err := f.doRequest(fmt.Sprintf("/repos/%s/%s/statuses/%s?limit=1",
		owner, repo, branchData.Commit.ID))
	if err != nil {
		return nil, err
	}
	defer statusResp.Body.Close()

	if statusResp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var statuses []fjCommitStatus
	if err := json.NewDecoder(statusResp.Body).Decode(&statuses); err != nil {
		return nil, err
	}

	if len(statuses) == 0 {
		return nil, nil
	}

	return &model.RepoCIStatus{
		Provider: "forgejo",
		Owner:    owner,
		Repo:     repo,
		Branch:   branch,
		State:    statuses[0].State,
	}, nil
}

func (f *ForgejoAdapter) ListOpenPRs(ctx context.Context, owner, repo string) (int, error) {
	resp, err := f.doRequest(fmt.Sprintf("/repos/%s/%s/pulls?state=open&limit=1", owner, repo))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var prs []fjPR
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return 0, err
	}
	return len(prs), nil
}
