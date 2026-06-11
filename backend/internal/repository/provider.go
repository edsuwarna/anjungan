package repository

import (
	"context"

	"github.com/edsuwarna/anjungan/internal/common/model"
)

// GitProvider is the interface that all git provider adapters must implement.
type GitProvider interface {
	// ListRepos returns all repositories accessible with this connection.
	ListRepos(ctx context.Context) ([]model.GitRepo, error)

	// ListBranches returns branches for a given repository.
	ListBranches(ctx context.Context, owner, repo string) ([]string, error)

	// GetCommitStatus returns the combined CI status for the default branch.
	GetCommitStatus(ctx context.Context, owner, repo, branch string) (*model.RepoCIStatus, error)

	// ListOpenPRs returns the count of open pull requests.
	ListOpenPRs(ctx context.Context, owner, repo string) (int, error)

	// Name returns the provider name ("github", "forgejo").
	Name() string

	// Validate checks that the token is valid by calling the provider API.
	Validate(ctx context.Context) (*ProviderIdentity, error)
}

// ProviderIdentity holds basic info about the authenticated user.
type ProviderIdentity struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// NewGitProvider creates the appropriate adapter based on provider type.
func NewGitProvider(providerType, token, baseURL, affiliations string) GitProvider {
	switch providerType {
	case "github":
		return &GitHubAdapter{token: token, affiliations: affiliations}
	case "forgejo":
		return &ForgejoAdapter{token: token, baseURL: baseURL, affiliations: affiliations}
	default:
		return nil
	}
}
