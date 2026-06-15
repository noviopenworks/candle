package store

import (
	"encoding/json"
	"strings"
)

// APISpec is a stored API contract's metadata.
type APISpec struct {
	ID      int64
	Kind    string
	Name    string
	Version string
	Path    string
}

// APISpecRow is an APISpec with its index_id.
type APISpecRow struct {
	APISpec
	IndexID int64
}

// HTTPOperation is a stored HTTP operation.
type HTTPOperation struct {
	Method         string
	Path           string
	OperationID    string
	Summary        string
	RequestSchema  string
	ResponseSchema string
	Security       []string
	Tags           []string
	SpecPath       string
}

// APISchema is a stored schema.
type APISchema struct {
	Name     string
	Kind     string
	RawRef   string
	SpecPath string
}

// APISpecBundle groups a spec with its operations and schemas for insertion.
type APISpecBundle struct {
	Spec       APISpec
	Operations []HTTPOperation
	Schemas    []APISchema
}

// ReplaceAPISpecs replaces all API specs (and their operations/schemas) for indexID.
func (s *Store) ReplaceAPISpecs(indexID int64, bundles []APISpecBundle) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Delete children then specs for this index.
	if _, err := tx.Exec(`DELETE FROM http_operations WHERE api_spec_id IN (SELECT id FROM api_specs WHERE index_id=?)`, indexID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM api_schemas WHERE api_spec_id IN (SELECT id FROM api_specs WHERE index_id=?)`, indexID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM api_specs WHERE index_id=?`, indexID); err != nil {
		return err
	}
	for _, b := range bundles {
		res, err := tx.Exec(`INSERT INTO api_specs(index_id, kind, name, version, path) VALUES(?,?,?,?,?)`,
			indexID, b.Spec.Kind, b.Spec.Name, b.Spec.Version, b.Spec.Path)
		if err != nil {
			return err
		}
		specID, _ := res.LastInsertId()
		for _, op := range b.Operations {
			if _, err := tx.Exec(
				`INSERT INTO http_operations(api_spec_id, method, path, operation_id, summary, request_schema, response_schema, security, tags)
				 VALUES(?,?,?,?,?,?,?,?,?)`,
				specID, op.Method, op.Path, op.OperationID, op.Summary, op.RequestSchema, op.ResponseSchema,
				jsonList(op.Security), jsonList(op.Tags)); err != nil {
				return err
			}
		}
		for _, sc := range b.Schemas {
			if _, err := tx.Exec(`INSERT INTO api_schemas(api_spec_id, name, kind, raw_ref) VALUES(?,?,?,?)`,
				specID, sc.Name, sc.Kind, sc.RawRef); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func jsonList(v []string) string {
	if len(v) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func parseList(s string) []string {
	var out []string
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

// ListAPISpecs returns specs for indexID.
func (s *Store) ListAPISpecs(indexID int64) ([]APISpecRow, error) {
	rows, err := s.DB.Query(`SELECT id, index_id, kind, COALESCE(name,''), COALESCE(version,''), path FROM api_specs WHERE index_id=?`, indexID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APISpecRow
	for rows.Next() {
		var r APISpecRow
		if err := rows.Scan(&r.ID, &r.IndexID, &r.Kind, &r.Name, &r.Version, &r.Path); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

const opCols = `o.method, o.path, COALESCE(o.operation_id,''), COALESCE(o.summary,''),
  COALESCE(o.request_schema,''), COALESCE(o.response_schema,''), COALESCE(o.security,''), COALESCE(o.tags,''), s.path`

func scanOps(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]HTTPOperation, error) {
	defer rows.Close()
	var out []HTTPOperation
	for rows.Next() {
		var o HTTPOperation
		var sec, tags string
		if err := rows.Scan(&o.Method, &o.Path, &o.OperationID, &o.Summary, &o.RequestSchema, &o.ResponseSchema, &sec, &tags, &o.SpecPath); err != nil {
			return nil, err
		}
		o.Security, o.Tags = parseList(sec), parseList(tags)
		out = append(out, o)
	}
	return out, rows.Err()
}

// FindOperations matches operations in indexID by operationId/path/method substring (case-insensitive).
func (s *Store) FindOperations(indexID int64, query string) ([]HTTPOperation, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT `+opCols+`
		FROM http_operations o JOIN api_specs s ON s.id=o.api_spec_id
		WHERE s.index_id=? AND (
		  LOWER(COALESCE(o.operation_id,'')) LIKE ? OR LOWER(o.path) LIKE ? OR LOWER(o.method) LIKE ? OR LOWER(COALESCE(o.summary,'')) LIKE ?)`,
		indexID, q, q, q, q)
	if err != nil {
		return nil, err
	}
	return scanOps(rows)
}

// OperationByMethodPath returns the operation matching method+path exactly.
func (s *Store) OperationByMethodPath(indexID int64, method, path string) (HTTPOperation, bool, error) {
	rows, err := s.DB.Query(`SELECT `+opCols+`
		FROM http_operations o JOIN api_specs s ON s.id=o.api_spec_id
		WHERE s.index_id=? AND UPPER(o.method)=UPPER(?) AND o.path=?`, indexID, method, path)
	if err != nil {
		return HTTPOperation{}, false, err
	}
	ops, err := scanOps(rows)
	if err != nil || len(ops) == 0 {
		return HTTPOperation{}, false, err
	}
	return ops[0], true, nil
}

// OperationByID returns the operation matching an operationId exactly.
func (s *Store) OperationByID(indexID int64, opID string) (HTTPOperation, bool, error) {
	rows, err := s.DB.Query(`SELECT `+opCols+`
		FROM http_operations o JOIN api_specs s ON s.id=o.api_spec_id
		WHERE s.index_id=? AND o.operation_id=?`, indexID, opID)
	if err != nil {
		return HTTPOperation{}, false, err
	}
	ops, err := scanOps(rows)
	if err != nil || len(ops) == 0 {
		return HTTPOperation{}, false, err
	}
	return ops[0], true, nil
}

// FindSchemas matches schemas in indexID by name substring (case-insensitive).
func (s *Store) FindSchemas(indexID int64, query string) ([]APISchema, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT sc.name, sc.kind, COALESCE(sc.raw_ref,''), s.path
		FROM api_schemas sc JOIN api_specs s ON s.id=sc.api_spec_id
		WHERE s.index_id=? AND LOWER(sc.name) LIKE ?`, indexID, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APISchema
	for rows.Next() {
		var sc APISchema
		if err := rows.Scan(&sc.Name, &sc.Kind, &sc.RawRef, &sc.SpecPath); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}
