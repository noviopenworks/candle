package store

import "strings"

// ProtoFile is a stored .proto file's metadata.
type ProtoFile struct {
	ID        int64
	Path      string
	Package   string
	GoPackage string
	Imports   []string
}

// ProtoFileRow is a ProtoFile with its index_id.
type ProtoFileRow struct {
	ProtoFile
	IndexID int64
}

// ProtoService is a stored gRPC service.
type ProtoService struct {
	ID       int64
	Name     string
	FullName string
}

// ProtoRPC is a stored RPC.
type ProtoRPC struct {
	Name            string
	FullName        string
	RequestMessage  string
	ResponseMessage string
	StreamKind      string
}

// ProtoRPCResult is an RPC joined with its service and file path.
type ProtoRPCResult struct {
	ProtoRPC
	Service   string
	ProtoPath string
}

// ProtoField is one message field.
type ProtoField struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Number int32  `json:"number"`
	Label  string `json:"label"`
}

// ProtoMessage is a stored message.
type ProtoMessage struct {
	Name     string
	FullName string
	Fields   []ProtoField
}

// ProtoMessageResult is a message joined with its file path.
type ProtoMessageResult struct {
	ProtoMessage
	ProtoPath string
}

// ProtoEnumValue is one enum value.
type ProtoEnumValue struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

// ProtoEnum is a stored enum.
type ProtoEnum struct {
	Name     string
	FullName string
	Values   []ProtoEnumValue
}

// ProtoServiceBundle groups a service with its RPCs.
type ProtoServiceBundle struct {
	Service ProtoService
	RPCs    []ProtoRPC
}

// ProtoFileBundle groups a file with its services, messages, and enums.
type ProtoFileBundle struct {
	File     ProtoFile
	Services []ProtoServiceBundle
	Messages []ProtoMessage
	Enums    []ProtoEnum
}

// ProtoRPCImpl is a same-repo implementation link for an RPC.
type ProtoRPCImpl struct {
	NodeID      string
	Confidence  float64
	MatchReason string
}

// RPCImplLink is an impl link keyed by RPC full name, written by the linker.
type RPCImplLink struct {
	RPCFullName string
	NodeID      string
	Confidence  float64
	MatchReason string
}

// ReplaceProtoFiles replaces all proto data (files/services/rpcs/messages/enums
// and impl links) for indexID. Idempotent.
func (s *Store) ReplaceProtoFiles(indexID int64, bundles []ProtoFileBundle) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`DELETE FROM proto_rpc_impls WHERE proto_rpc_id IN (SELECT r.id FROM proto_rpcs r
		   JOIN proto_services sv ON sv.id=r.proto_service_id
		   JOIN proto_files f ON f.id=sv.proto_file_id WHERE f.index_id=?)`,
		`DELETE FROM proto_rpcs WHERE proto_service_id IN (SELECT sv.id FROM proto_services sv
		   JOIN proto_files f ON f.id=sv.proto_file_id WHERE f.index_id=?)`,
		`DELETE FROM proto_services WHERE proto_file_id IN (SELECT id FROM proto_files WHERE index_id=?)`,
		`DELETE FROM proto_messages WHERE proto_file_id IN (SELECT id FROM proto_files WHERE index_id=?)`,
		`DELETE FROM proto_enums WHERE proto_file_id IN (SELECT id FROM proto_files WHERE index_id=?)`,
		`DELETE FROM proto_files WHERE index_id=?`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q, indexID); err != nil {
			return err
		}
	}
	for _, b := range bundles {
		res, err := tx.Exec(`INSERT INTO proto_files(index_id, path, package, go_package, imports) VALUES(?,?,?,?,?)`,
			indexID, b.File.Path, b.File.Package, b.File.GoPackage, jsonList(b.File.Imports))
		if err != nil {
			return err
		}
		fileID, _ := res.LastInsertId()
		for _, sb := range b.Services {
			sres, err := tx.Exec(`INSERT INTO proto_services(proto_file_id, name, full_name) VALUES(?,?,?)`,
				fileID, sb.Service.Name, sb.Service.FullName)
			if err != nil {
				return err
			}
			svcID, _ := sres.LastInsertId()
			for _, r := range sb.RPCs {
				if _, err := tx.Exec(`INSERT INTO proto_rpcs(proto_service_id, name, full_name, request_message, response_message, stream_kind)
					VALUES(?,?,?,?,?,?)`, svcID, r.Name, r.FullName, r.RequestMessage, r.ResponseMessage, r.StreamKind); err != nil {
					return err
				}
			}
		}
		for _, m := range b.Messages {
			if _, err := tx.Exec(`INSERT INTO proto_messages(proto_file_id, name, full_name, fields) VALUES(?,?,?,?)`,
				fileID, m.Name, m.FullName, jsonBlob(m.Fields)); err != nil {
				return err
			}
		}
		for _, e := range b.Enums {
			if _, err := tx.Exec(`INSERT INTO proto_enums(proto_file_id, name, full_name, "values") VALUES(?,?,?,?)`,
				fileID, e.Name, e.FullName, jsonBlob(e.Values)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// LinkRPCImpls replaces all impl links for indexID, resolving RPC full names to
// proto_rpcs rows in this index. Unmatched RPC names are ignored.
func (s *Store) LinkRPCImpls(indexID int64, links []RPCImplLink) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM proto_rpc_impls WHERE proto_rpc_id IN (SELECT r.id FROM proto_rpcs r
		JOIN proto_services sv ON sv.id=r.proto_service_id
		JOIN proto_files f ON f.id=sv.proto_file_id WHERE f.index_id=?)`, indexID); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.Exec(`INSERT INTO proto_rpc_impls(proto_rpc_id, node_id, confidence, match_reason)
			SELECT r.id, ?, ?, ? FROM proto_rpcs r
			  JOIN proto_services sv ON sv.id=r.proto_service_id
			  JOIN proto_files f ON f.id=sv.proto_file_id
			WHERE f.index_id=? AND r.full_name=?`,
			l.NodeID, l.Confidence, l.MatchReason, indexID, l.RPCFullName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListProtoFiles returns proto files for indexID.
func (s *Store) ListProtoFiles(indexID int64) ([]ProtoFileRow, error) {
	rows, err := s.DB.Query(`SELECT id, index_id, path, COALESCE(package,''), COALESCE(go_package,''), COALESCE(imports,'')
		FROM proto_files WHERE index_id=?`, indexID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtoFileRow
	for rows.Next() {
		var r ProtoFileRow
		var imports string
		if err := rows.Scan(&r.ID, &r.IndexID, &r.Path, &r.Package, &r.GoPackage, &imports); err != nil {
			return nil, err
		}
		r.Imports = parseList(imports)
		out = append(out, r)
	}
	return out, rows.Err()
}

const rpcCols = `r.name, r.full_name, COALESCE(r.request_message,''), COALESCE(r.response_message,''),
  r.stream_kind, sv.name, f.path`

func scanRPCs(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]ProtoRPCResult, error) {
	defer rows.Close()
	var out []ProtoRPCResult
	for rows.Next() {
		var r ProtoRPCResult
		if err := rows.Scan(&r.Name, &r.FullName, &r.RequestMessage, &r.ResponseMessage, &r.StreamKind, &r.Service, &r.ProtoPath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FindRPCs matches RPCs in indexID by name/service/full_name substring,
// optionally filtered to a stream_kind (empty = any).
func (s *Store) FindRPCs(indexID int64, query, streamKind string) ([]ProtoRPCResult, error) {
	q := "%" + strings.ToLower(query) + "%"
	sql := `SELECT ` + rpcCols + ` FROM proto_rpcs r
		JOIN proto_services sv ON sv.id=r.proto_service_id
		JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE f.index_id=? AND (LOWER(r.name) LIKE ? OR LOWER(r.full_name) LIKE ? OR LOWER(sv.name) LIKE ?)`
	args := []any{indexID, q, q, q}
	if streamKind != "" {
		sql += ` AND r.stream_kind=?`
		args = append(args, streamKind)
	}
	rows, err := s.DB.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	return scanRPCs(rows)
}

// RPCByServiceName returns the RPC matching service + rpc name exactly.
func (s *Store) RPCByServiceName(indexID int64, service, rpc string) (ProtoRPCResult, bool, error) {
	rows, err := s.DB.Query(`SELECT `+rpcCols+` FROM proto_rpcs r
		JOIN proto_services sv ON sv.id=r.proto_service_id
		JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE f.index_id=? AND sv.name=? AND r.name=?`, indexID, service, rpc)
	if err != nil {
		return ProtoRPCResult{}, false, err
	}
	got, err := scanRPCs(rows)
	if err != nil || len(got) == 0 {
		return ProtoRPCResult{}, false, err
	}
	return got[0], true, nil
}

// FindMessages matches messages in indexID by name substring.
func (s *Store) FindMessages(indexID int64, query string) ([]ProtoMessageResult, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT m.name, m.full_name, COALESCE(m.fields,''), f.path
		FROM proto_messages m JOIN proto_files f ON f.id=m.proto_file_id
		WHERE f.index_id=? AND (LOWER(m.name) LIKE ? OR LOWER(m.full_name) LIKE ?)`, indexID, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtoMessageResult
	for rows.Next() {
		var m ProtoMessageResult
		var fields string
		if err := rows.Scan(&m.Name, &m.FullName, &fields, &m.ProtoPath); err != nil {
			return nil, err
		}
		m.Fields = parseFields(fields)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ProtoRPCImpls returns impl links for an RPC full name in indexID.
func (s *Store) ProtoRPCImpls(indexID int64, rpcFullName string) ([]ProtoRPCImpl, error) {
	rows, err := s.DB.Query(`SELECT i.node_id, i.confidence, COALESCE(i.match_reason,'')
		FROM proto_rpc_impls i JOIN proto_rpcs r ON r.id=i.proto_rpc_id
		  JOIN proto_services sv ON sv.id=r.proto_service_id
		  JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE f.index_id=? AND r.full_name=?`, indexID, rpcFullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProtoRPCImpl
	for rows.Next() {
		var im ProtoRPCImpl
		if err := rows.Scan(&im.NodeID, &im.Confidence, &im.MatchReason); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

// ProtoRPCDefiningIndexes returns the distinct index ids whose protobuf defines
// an RPC with the given full name. Used to exclude providers from cross-repo
// consumer aggregation: a repo defining an RPC is a provider, not a consumer.
func (s *Store) ProtoRPCDefiningIndexes(rpcFullName string) ([]int64, error) {
	rows, err := s.DB.Query(`
		SELECT DISTINCT f.index_id
		FROM proto_rpcs r
		  JOIN proto_services sv ON sv.id=r.proto_service_id
		  JOIN proto_files f ON f.id=sv.proto_file_id
		WHERE r.full_name=?`, rpcFullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
