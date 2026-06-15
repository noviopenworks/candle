# Tasks — go-private-library-layer

> Open-phase outline. Refined against the Design Doc + delta specs.

## 1. Storage
- [ ] 1.1 Add `dependencies` (ecosystem, is_private), `private_library_exports`, `private_library_usages` tables; migration

## 2. Dependency parsing
- [ ] 2.1 Parse `go.mod`/`go.work` with `x/mod/modfile`; record require/replace + versions
- [ ] 2.2 Cross-check versions via `go.sum`
- [ ] 2.3 Config-driven private classification (internal module-path prefixes via viper)

## 3. Provider side (exports)
- [ ] 3.1 Extract exported packages/functions/types/interfaces/constructors for private modules
- [ ] 3.2 Persist to `private_library_exports`

## 4. Consumer side (usages)
- [ ] 4.1 Detect imports of private modules per repo (with version)
- [ ] 4.2 Resolve used exported symbols (design-phase fidelity) with file/line → `private_library_usages`

## 5. Tools
- [ ] 5.1 `find_private_library` (name / module-path / purpose)
- [ ] 5.2 `find_library_consumers` (repos + versions + used_symbols)

## 6. Resources
- [ ] 6.1 `lib://<module-path>` and `/version/`, `/package/`, `/symbol/` variants

## 7. Verification
- [ ] 7.1 Provider repo: exports indexed correctly
- [ ] 7.2 Consumer repos: `find_library_consumers` lists repos + versions + used symbols on a multi-repo fixture
- [ ] 7.3 Public deps are classified non-private and not deeply indexed
