## Fix

The go-sdk routes a `resources/read` URI to a template handler only if
`uritemplate.New(template).Regexp().MatchString(uri)` is true. RFC 6570 *simple*
string expansion (`{var}`) generates a regexp that excludes reserved characters,
including `/`. **Reserved expansion** (`{+var}`) includes them.

So the one-line-per-template fix is to mark the slash-bearing trailing variables
as reserved:

| Template (before) | Template (after) |
|---|---|
| `openapi://{org}/{name}/commit/{sha}/{kind}/{ref}` | `…/{kind}/{+ref}` |
| `proto://{org}/{name}/commit/{sha}/{kind}/{ref}` | `…/{kind}/{+ref}` |
| `lib://{module}` | `lib://{+module}` |
| `graph://{org}/{name}/commit/{sha}/node/{nodeID}` | `…/node/{+nodeID}` |

The handlers already pass `req.Params.URI` (the full URI) to `parseProtoURI` /
`parseOpenAPIURI` / `parseLibURI` / `parseGraphNodeURI`, which split on `/`
themselves — so no handler/parse change is needed. The `{+var}` change only
affects whether the URI **routes** to the handler.

Verified empirically with `yosida95/uritemplate/v3` (the SDK's matcher):
`{ref}` → `MatchString` false on a 3-segment ref; `{+ref}` → true.

## Testing

The existing e2e (`TestEndToEndToolSurface`) already probes all five resource
schemes. Its `mustErr` characterization guards for the proto rpc/file and lib
URIs flip to `mustContain` assertions, turning the documented limitation into
positive coverage. Lightweight verification: `go build/vet/test ./...`.

## Risk

Minimal. Reserved expansion is strictly more permissive in matching; single-segment
refs (operations, node ids) still match. No behavior change beyond enabling the
previously-unroutable reads.
