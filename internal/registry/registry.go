package registry

import (
	"strings"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// RepoInfo describes a resolved repo snapshot.
type RepoInfo struct {
	IndexID    int64
	Repo       string // org/name
	Branch     string
	Commit     string
	IngestedAt string
	NodeCount  int
}

// Registry resolves repo identities to indexed snapshots.
type Registry struct{ s *store.Store }

// New builds a Registry over the store.
func New(s *store.Store) *Registry { return &Registry{s: s} }

// List returns all indexed repo snapshots.
func (r *Registry) List() ([]RepoInfo, error) {
	rows, err := r.s.DB.Query(`
		SELECT i.id, r.org, r.name, COALESCE(i.branch,''), COALESCE(i.commit_sha,''), i.ingested_at,
		       (SELECT COUNT(*) FROM nodes n WHERE n.index_id=i.id)
		FROM indexes i JOIN repos r ON r.id=i.repo_id
		ORDER BY r.org, r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RepoInfo
	for rows.Next() {
		var ri RepoInfo
		var org, name string
		if err := rows.Scan(&ri.IndexID, &org, &name, &ri.Branch, &ri.Commit, &ri.IngestedAt, &ri.NodeCount); err != nil {
			return nil, err
		}
		ri.Repo = org + "/" + name
		out = append(out, ri)
	}
	return out, rows.Err()
}

// Resolve returns the snapshot for an exact org/name identity.
func (r *Registry) Resolve(repo string) (RepoInfo, bool, error) {
	all, err := r.List()
	if err != nil {
		return RepoInfo{}, false, err
	}
	for _, ri := range all {
		if ri.Repo == repo {
			return ri, true, nil
		}
	}
	return RepoInfo{}, false, nil
}

// Match returns snapshots whose repo identity contains the query substring,
// case-insensitively (simple fuzzy match).
func (r *Registry) Match(query string) ([]RepoInfo, error) {
	all, err := r.List()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []RepoInfo
	for _, ri := range all {
		if strings.Contains(strings.ToLower(ri.Repo), q) {
			out = append(out, ri)
		}
	}
	return out, nil
}
