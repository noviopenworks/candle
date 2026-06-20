package store

import (
	"errors"
	"strings"
)

// Dependency is a stored module dependency.
type Dependency struct {
	ModulePath string
	Version    string
	Ecosystem  string
	IsPrivate  bool
	Direct     bool
}

// PrivateExport is one exported symbol of a private library.
type PrivateExport struct {
	PackagePath string
	Symbol      string
	Kind        string
	Doc         string
	NodeID      string
}

// PrivateLibrary is a provider module's metadata.
type PrivateLibrary struct {
	ID          int64
	ModulePath  string
	Readme      string
	DocSynopsis string
}

// PrivateLibraryBundle groups a library with its exports for insertion.
type PrivateLibraryBundle struct {
	Library PrivateLibrary
	Exports []PrivateExport
}

// PrivateLibraryRow is a library plus its exports (read side).
type PrivateLibraryRow struct {
	PrivateLibrary
	IndexID int64
	Exports []PrivateExport
}

// PrivateLibraryResult is a find_private_library match.
type PrivateLibraryResult struct {
	ModulePath  string
	Packages    []string
	ExportCount int
	DocSynopsis string
}

// PrivateUsage is a consumer's use of a private module symbol.
type PrivateUsage struct {
	ModulePath  string
	Version     string
	PackagePath string
	Symbol      string
	File        string
	Line        int
}

// GoDepBundle is the full Go dependency data for one index.
type GoDepBundle struct {
	Dependencies []Dependency
	Libraries    []PrivateLibraryBundle
	Usages       []PrivateUsage
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ReplaceGoDeps replaces all Go dependency data for indexID. Idempotent.
func (s *Store) ReplaceGoDeps(indexID int64, b GoDepBundle) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`DELETE FROM private_library_exports WHERE private_library_id IN (SELECT id FROM private_libraries WHERE index_id=?)`,
		`DELETE FROM private_libraries WHERE index_id=?`,
		`DELETE FROM private_library_usages WHERE index_id=?`,
		`DELETE FROM dependencies WHERE index_id=?`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q, indexID); err != nil {
			return err
		}
	}
	for _, d := range b.Dependencies {
		if _, err := tx.Exec(`INSERT INTO dependencies(index_id, module_path, version, ecosystem, is_private, direct) VALUES(?,?,?,?,?,?)`,
			indexID, d.ModulePath, d.Version, d.Ecosystem, boolToInt(d.IsPrivate), boolToInt(d.Direct)); err != nil {
			return err
		}
	}
	for _, lb := range b.Libraries {
		res, err := tx.Exec(`INSERT INTO private_libraries(index_id, module_path, readme, doc_synopsis) VALUES(?,?,?,?)`,
			indexID, lb.Library.ModulePath, lb.Library.Readme, lb.Library.DocSynopsis)
		if err != nil {
			return err
		}
		libID, _ := res.LastInsertId()
		for _, e := range lb.Exports {
			if _, err := tx.Exec(`INSERT INTO private_library_exports(private_library_id, package_path, symbol, kind, doc, node_id) VALUES(?,?,?,?,?,?)`,
				libID, e.PackagePath, e.Symbol, e.Kind, e.Doc, e.NodeID); err != nil {
				return err
			}
		}
	}
	for _, u := range b.Usages {
		if _, err := tx.Exec(`INSERT INTO private_library_usages(index_id, module_path, version, package_path, symbol, file, line) VALUES(?,?,?,?,?,?,?)`,
			indexID, u.ModulePath, u.Version, u.PackagePath, u.Symbol, u.File, u.Line); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) exportsByLib(libID int64) ([]PrivateExport, error) {
	rows, err := s.DB.Query(`SELECT package_path, symbol, kind, COALESCE(doc,''), COALESCE(node_id,'')
		FROM private_library_exports WHERE private_library_id=?`, libID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrivateExport
	for rows.Next() {
		var e PrivateExport
		if err := rows.Scan(&e.PackagePath, &e.Symbol, &e.Kind, &e.Doc, &e.NodeID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FindPrivateLibraries matches provider libraries in indexID by module path,
// doc synopsis, readme, or any export package path (case-insensitive).
func (s *Store) FindPrivateLibraries(indexID int64, query string) ([]PrivateLibraryResult, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT id, module_path, COALESCE(doc_synopsis,'') FROM private_libraries
		WHERE index_id=? AND (LOWER(module_path) LIKE ? OR LOWER(COALESCE(doc_synopsis,'')) LIKE ? OR LOWER(COALESCE(readme,'')) LIKE ?
		  OR id IN (SELECT private_library_id FROM private_library_exports WHERE LOWER(package_path) LIKE ?))`,
		indexID, q, q, q, q)
	if err != nil {
		return nil, err
	}
	// Collect the library rows and close the cursor before issuing the nested
	// exportsByLib queries below: holding an open cursor while querying again
	// can grab a second pooled connection (which, for ":memory:", is a separate
	// schema-less database).
	type libHit struct {
		id int64
		r  PrivateLibraryResult
	}
	var hits []libHit
	for rows.Next() {
		var h libHit
		if err := rows.Scan(&h.id, &h.r.ModulePath, &h.r.DocSynopsis); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				return nil, errors.Join(err, closeErr)
			}
			return nil, err
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var out []PrivateLibraryResult
	for _, h := range hits {
		exps, err := s.exportsByLib(h.id)
		if err != nil {
			return nil, err
		}
		r := h.r
		r.ExportCount = len(exps)
		seen := map[string]bool{}
		for _, e := range exps {
			if !seen[e.PackagePath] {
				seen[e.PackagePath] = true
				r.Packages = append(r.Packages, e.PackagePath)
			}
		}
		out = append(out, r)
	}
	return out, nil
}

// FindPrivateDeps returns private dependencies in indexID whose module path
// matches query (path-only matches for find_private_library).
func (s *Store) FindPrivateDeps(indexID int64, query string) ([]Dependency, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT module_path, COALESCE(version,''), ecosystem, is_private, direct
		FROM dependencies WHERE index_id=? AND is_private=1 AND LOWER(module_path) LIKE ?`, indexID, q)
	if err != nil {
		return nil, err
	}
	return scanDeps(rows)
}

func scanDeps(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]Dependency, error) {
	defer rows.Close()
	var out []Dependency
	for rows.Next() {
		var d Dependency
		var priv, direct int
		if err := rows.Scan(&d.ModulePath, &d.Version, &d.Ecosystem, &priv, &direct); err != nil {
			return nil, err
		}
		d.IsPrivate, d.Direct = priv == 1, direct == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

// DependencyByModule returns the dependency for a module path in indexID.
func (s *Store) DependencyByModule(indexID int64, modulePath string) (Dependency, bool, error) {
	rows, err := s.DB.Query(`SELECT module_path, COALESCE(version,''), ecosystem, is_private, direct
		FROM dependencies WHERE index_id=? AND module_path=?`, indexID, modulePath)
	if err != nil {
		return Dependency{}, false, err
	}
	deps, err := scanDeps(rows)
	if err != nil || len(deps) == 0 {
		return Dependency{}, false, err
	}
	return deps[0], true, nil
}

// PrivateUsagesByModule returns consumer usages of a module in indexID.
func (s *Store) PrivateUsagesByModule(indexID int64, modulePath string) ([]PrivateUsage, error) {
	rows, err := s.DB.Query(`SELECT module_path, COALESCE(version,''), package_path, COALESCE(symbol,''), COALESCE(file,''), COALESCE(line,0)
		FROM private_library_usages WHERE index_id=? AND module_path=?`, indexID, modulePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrivateUsage
	for rows.Next() {
		var u PrivateUsage
		if err := rows.Scan(&u.ModulePath, &u.Version, &u.PackagePath, &u.Symbol, &u.File, &u.Line); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// PrivateLibraryByModule returns the provider library (with exports) for a module
// path, searched store-wide (the defining repo is unique). For lib:// resources.
func (s *Store) PrivateLibraryByModule(modulePath string) (PrivateLibraryRow, bool, error) {
	rows, err := s.DB.Query(`SELECT id, index_id, module_path, COALESCE(readme,''), COALESCE(doc_synopsis,'')
		FROM private_libraries WHERE module_path=? LIMIT 1`, modulePath)
	if err != nil {
		return PrivateLibraryRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return PrivateLibraryRow{}, false, rows.Err()
	}
	var r PrivateLibraryRow
	if err := rows.Scan(&r.ID, &r.IndexID, &r.ModulePath, &r.Readme, &r.DocSynopsis); err != nil {
		return PrivateLibraryRow{}, false, err
	}
	if err := rows.Close(); err != nil {
		return PrivateLibraryRow{}, false, err
	}
	exps, err := s.exportsByLib(r.ID)
	if err != nil {
		return PrivateLibraryRow{}, false, err
	}
	r.Exports = exps
	return r, true, nil
}

// RepoConsumer is one repo's consumption of a private module (cross-repo aggregation).
type RepoConsumer struct {
	IndexID      int64          `json:"-"`
	Repo         string         `json:"repo"`
	Commit       string         `json:"commit"`
	Version      string         `json:"version"`
	UsedPackages []string       `json:"used_packages"`
	UsedSymbols  []PrivateUsage `json:"used_symbols"`
}

// PrivateConsumersAcrossRepos aggregates, across all indexes, every repo that
// uses or depends on modulePath. It collects the consuming index ids first
// (closing the cursor) and then reuses index-scoped helpers, per the :memory:
// pooled-connection caveat.
func (s *Store) PrivateConsumersAcrossRepos(modulePath string) ([]RepoConsumer, error) {
	rows, err := s.DB.Query(`
		SELECT DISTINCT i.id, r.org, r.name, COALESCE(i.commit_sha,'')
		FROM indexes i JOIN repos r ON r.id=i.repo_id
		WHERE i.id IN (SELECT index_id FROM private_library_usages WHERE module_path=?)
		   OR i.id IN (SELECT index_id FROM dependencies WHERE module_path=? AND is_private=1)
		ORDER BY r.org, r.name`, modulePath, modulePath)
	if err != nil {
		return nil, err
	}
	type ident struct {
		id     int64
		repo   string
		commit string
	}
	var idents []ident
	for rows.Next() {
		var it ident
		var org, name string
		if err := rows.Scan(&it.id, &org, &name, &it.commit); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				return nil, errors.Join(err, closeErr)
			}
			return nil, err
		}
		it.repo = org + "/" + name
		idents = append(idents, it)
	}
	if err := rows.Err(); err != nil {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var out []RepoConsumer
	for _, it := range idents {
		usages, err := s.PrivateUsagesByModule(it.id, modulePath)
		if err != nil {
			return nil, err
		}
		rc := RepoConsumer{IndexID: it.id, Repo: it.repo, Commit: it.commit, UsedSymbols: usages}
		if dep, found, err := s.DependencyByModule(it.id, modulePath); err != nil {
			return nil, err
		} else if found {
			rc.Version = dep.Version
		}
		if rc.Version == "" {
			for _, u := range usages {
				if u.Version != "" {
					rc.Version = u.Version
					break
				}
			}
		}
		seen := map[string]bool{}
		for _, u := range usages {
			if u.PackagePath != "" && !seen[u.PackagePath] {
				seen[u.PackagePath] = true
				rc.UsedPackages = append(rc.UsedPackages, u.PackagePath)
			}
		}
		out = append(out, rc)
	}
	return out, nil
}

// SearchPrivateModulePaths returns distinct private module paths across all
// indexes whose module path, doc synopsis, readme, or package path matches
// query, plus path-only private dependencies matching by module path.
func (s *Store) SearchPrivateModulePaths(query string) ([]string, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`
		SELECT module_path FROM private_libraries
		WHERE LOWER(module_path) LIKE ? OR LOWER(COALESCE(doc_synopsis,'')) LIKE ? OR LOWER(COALESCE(readme,'')) LIKE ?
		   OR id IN (SELECT private_library_id FROM private_library_exports WHERE LOWER(package_path) LIKE ?)
		UNION
		SELECT module_path FROM dependencies WHERE is_private=1 AND LOWER(module_path) LIKE ?`,
		q, q, q, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var mp string
		if err := rows.Scan(&mp); err != nil {
			return nil, err
		}
		if !seen[mp] {
			seen[mp] = true
			out = append(out, mp)
		}
	}
	return out, rows.Err()
}
