package graph

import "github.com/noviopenworks/candlegraph/internal/store"

// LoadResult reports how many rows were ingested.
type LoadResult struct {
	Nodes, Edges, Hyperedges, Skipped int
}

// Load ingests g into the store under indexID. It is idempotent: existing rows
// for indexID are deleted first, then re-inserted in one transaction. Malformed
// entries (e.g. nodes without an id, edges without endpoints) are skipped.
func Load(s *store.Store, indexID int64, g *Graph) (LoadResult, error) {
	var res LoadResult
	tx, err := s.DB.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM nodes WHERE index_id=?", indexID); err != nil {
		return res, err
	}
	if _, err := tx.Exec("DELETE FROM edges WHERE index_id=?", indexID); err != nil {
		return res, err
	}
	if _, err := tx.Exec("DELETE FROM hyperedges WHERE index_id=?", indexID); err != nil {
		return res, err
	}
	if _, err := tx.Exec("DELETE FROM hyperedge_members WHERE index_id=?", indexID); err != nil {
		return res, err
	}

	for _, n := range g.Nodes {
		if n.ID == "" {
			res.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO nodes(index_id, node_id, label, file_type, source_file, source_location, source_url, captured_at, author, contributor)
			 VALUES(?,?,?,?,?,?,?,?,?,?)`,
			indexID, n.ID, n.Label, n.FileType, n.SourceFile, n.SourceLocation, n.SourceURL, n.CapturedAt, n.Author, n.Contributor); err != nil {
			return res, err
		}
		res.Nodes++
	}
	for _, e := range g.Edges {
		if e.Source == "" || e.Target == "" || e.Relation == "" {
			res.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO edges(index_id, source, target, relation, confidence, confidence_score, weight, source_file)
			 VALUES(?,?,?,?,?,?,?,?)`,
			indexID, e.Source, e.Target, e.Relation, e.Confidence, e.ConfidenceScore, e.Weight, e.SourceFile); err != nil {
			return res, err
		}
		res.Edges++
	}
	for _, h := range g.Hyperedges {
		if h.ID == "" {
			res.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO hyperedges(index_id, hyperedge_id, label, relation, confidence, confidence_score, source_file)
			 VALUES(?,?,?,?,?,?,?)`,
			indexID, h.ID, h.Label, h.Relation, h.Confidence, h.ConfidenceScore, h.SourceFile); err != nil {
			return res, err
		}
		for _, m := range h.Nodes {
			if _, err := tx.Exec(
				`INSERT INTO hyperedge_members(index_id, hyperedge_id, node_id) VALUES(?,?,?)`,
				indexID, h.ID, m); err != nil {
				return res, err
			}
		}
		res.Hyperedges++
	}
	return res, tx.Commit()
}
