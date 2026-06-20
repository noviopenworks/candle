## Why

The project now produces installable artifacts — `go install` works and tagged
releases publish cross-platform binaries (`add-ci-pipeline`) — but the docs still
only tell users to `go build` from a clone. There is **no documented install
path** for the two supported methods the locked decisions call for (`go install`
+ release binaries), and the `mise`/`Task` developer workflow added earlier is
undocumented. Separately, `flow.md` (an end-to-end "how it works" narrative) is
committed but **linked from nowhere**, so readers never find it.

## What Changes

- Add an **Installation** section near the top of **`README.md`**: `go install
  …@latest`, prebuilt release binaries, and a from-source/`mise`+`task` path.
- Update **`docs/getting-started.md`**: prerequisites now mention `mise` and the
  current Go pin (1.26.4); the "Build" step becomes "Install or build" covering
  `go install`, release binaries, source build, and `task install`.
- Wire **`flow.md`** into discovery: add it to the README documentation table and
  to `docs/README.md` (a reading path + the index).
- Add a short `mise`/`task ci` note to **`CONTRIBUTING.md`** so contributors run
  the same gate CI runs.

## Capabilities

### Modified Capabilities
<!-- None. Documentation only; no code, build, test, or schema change. -->

## Impact

- **Files:** `README.md`, `docs/getting-started.md`, `docs/README.md`,
  `CONTRIBUTING.md` (all docs); `flow.md` now linked (content unchanged).
- **No code change.** `go build ./...` / `go test ./...` (116) unchanged.
- Closes out the "proper project" effort: docs, hardening, install, CI, and
  tooling are all in place.
