package registry

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/noviopenworks/candlegraph/internal/config"
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

// Registry resolves repo identities to indexed snapshots, optionally scoped to
// an allow-set of index ids (nil = unscoped, serve all).
type Registry struct {
	s       *store.Store
	allowed map[int64]bool
}

// New builds an unscoped Registry over the store.
func New(s *store.Store) *Registry { return &Registry{s: s} }

// NewScoped builds a Registry limited to the given index ids.
func NewScoped(s *store.Store, allowed map[int64]bool) *Registry {
	return &Registry{s: s, allowed: allowed}
}

// BuildScope resolves manifest repo entries to an allow-set of index ids.
// Pinned commits select the exact snapshot; omitted commits select the latest
// indexed snapshot by ingested_at. Missing entries produce warnings, not errors.
func BuildScope(s *store.Store, cfg *config.Config) (map[int64]bool, []string, error) {
	allowed := map[int64]bool{}
	var warns []string
	for _, rc := range cfg.Repos {
		var (
			indexID int64
			err     error
		)
		if rc.Commit != "" {
			err = s.DB.QueryRow(`
				SELECT i.id
				FROM indexes i JOIN repos r ON r.id=i.repo_id
				WHERE r.org=? AND r.name=? AND i.commit_sha=?`, rc.Org(), rc.Name(), rc.Commit).Scan(&indexID)
		} else {
			err = s.DB.QueryRow(`
				SELECT i.id
				FROM indexes i JOIN repos r ON r.id=i.repo_id
				WHERE r.org=? AND r.name=?
				ORDER BY i.ingested_at DESC, i.id DESC
				LIMIT 1`, rc.Org(), rc.Name()).Scan(&indexID)
		}
		if err == nil {
			allowed[indexID] = true
			continue
		}
		if err != sql.ErrNoRows {
			return nil, nil, err
		}
		if rc.Commit != "" {
			warns = append(warns, fmt.Sprintf("%s: commit %s not indexed", rc.Repo, rc.Commit))
		} else {
			warns = append(warns, fmt.Sprintf("%s: no indexed snapshot in store", rc.Repo))
		}
	}
	return allowed, warns, nil
}

// InScope reports whether an index id is served. Unscoped registries serve all.
func (r *Registry) InScope(indexID int64) bool {
	if r.allowed == nil {
		return true
	}
	return r.allowed[indexID]
}

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
		if !r.InScope(ri.IndexID) {
			continue
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
