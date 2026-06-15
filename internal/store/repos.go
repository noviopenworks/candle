package store

import "time"

// UpsertIndex ensures a repo row and an index (snapshot) row exist for
// (org/name, commit), returning the index_id. Idempotent on (repo, commit).
func (s *Store) UpsertIndex(org, name, commit, branch, graphPath string) (int64, error) {
	if _, err := s.DB.Exec(
		`INSERT INTO repos(org, name) VALUES(?, ?)
		 ON CONFLICT(org, name) DO NOTHING`, org, name); err != nil {
		return 0, err
	}
	var repoID int64
	if err := s.DB.QueryRow(
		`SELECT id FROM repos WHERE org=? AND name=?`, org, name).Scan(&repoID); err != nil {
		return 0, err
	}
	if _, err := s.DB.Exec(
		`INSERT INTO indexes(repo_id, commit_sha, branch, graph_path, ingested_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(repo_id, commit_sha)
		 DO UPDATE SET branch=excluded.branch, graph_path=excluded.graph_path, ingested_at=excluded.ingested_at`,
		repoID, commit, branch, graphPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return 0, err
	}
	var indexID int64
	if err := s.DB.QueryRow(
		`SELECT id FROM indexes WHERE repo_id=? AND commit_sha=?`, repoID, commit).Scan(&indexID); err != nil {
		return 0, err
	}
	return indexID, nil
}
