package store

// NodeRow is a stored graph node.
type NodeRow struct {
	IndexID        int64
	NodeID         string
	Label          string
	FileType       string
	SourceFile     string
	SourceLocation string
}

// EdgeRow is a stored graph edge.
type EdgeRow struct {
	Source   string
	Target   string
	Relation string
}

func scanNodes(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]NodeRow, error) {
	defer rows.Close()
	var out []NodeRow
	for rows.Next() {
		var n NodeRow
		if err := rows.Scan(&n.IndexID, &n.NodeID, &n.Label, &n.FileType, &n.SourceFile, &n.SourceLocation); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

const nodeCols = `index_id, node_id, COALESCE(label,''), COALESCE(file_type,''), COALESCE(source_file,''), COALESCE(source_location,'')`

// NodesByLabel returns nodes in indexID whose label matches exactly.
func (s *Store) NodesByLabel(indexID int64, label string) ([]NodeRow, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE index_id=? AND label=?`, indexID, label)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}

// NodeByID returns a single node by id, or (zero,false) if absent.
func (s *Store) NodeByID(indexID int64, nodeID string) (NodeRow, bool, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE index_id=? AND node_id=?`, indexID, nodeID)
	if err != nil {
		return NodeRow{}, false, err
	}
	ns, err := scanNodes(rows)
	if err != nil || len(ns) == 0 {
		return NodeRow{}, false, err
	}
	return ns[0], true, nil
}

// Callees returns edges where nodeID is the source.
func (s *Store) Callees(indexID int64, nodeID string) ([]EdgeRow, error) {
	rows, err := s.DB.Query(`SELECT source, target, relation FROM edges WHERE index_id=? AND source=?`, indexID, nodeID)
	if err != nil {
		return nil, err
	}
	return scanEdges(rows)
}

// Callers returns edges where nodeID is the target.
func (s *Store) Callers(indexID int64, nodeID string) ([]EdgeRow, error) {
	rows, err := s.DB.Query(`SELECT source, target, relation FROM edges WHERE index_id=? AND target=?`, indexID, nodeID)
	if err != nil {
		return nil, err
	}
	return scanEdges(rows)
}

func scanEdges(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]EdgeRow, error) {
	defer rows.Close()
	var out []EdgeRow
	for rows.Next() {
		var e EdgeRow
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// NodesByLabelAllIndexes is the cross-index helper downstream layers use for
// cross-repo relationships. It matches a label across every index.
func (s *Store) NodesByLabelAllIndexes(label string) ([]NodeRow, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE label=?`, label)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}

// NodesByFile returns nodes whose source_file matches in indexID.
func (s *Store) NodesByFile(indexID int64, file string) ([]NodeRow, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE index_id=? AND source_file=?`, indexID, file)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}
