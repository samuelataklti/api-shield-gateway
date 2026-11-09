# API Shield Gateway — Modular Source Refactor

## Scope

This package splits the supplied single-file `main.go` into nine Go source files within the **same `package main`**, and adds `features_test.go` containing four regression tests.

**Keep your current `go.mod`, `go.sum`, and model file.** They are intentionally NOT bundled here. Keep the model named `api_threat_model_lgbm_antioverfit_strict_12_v3.txt` in the repository root, because that is the default model path in `inference.go`.

## File ownership

- `main.go`: server setup and HTTP route registration
- `config.go`: constants, flags and environment-value helpers
- `gateway.go`: gateway request lifecycle and response headers
- `features.go`: STRICT-12 feature extraction and payload parsing
- `session.go`: concurrency-safe session state and target tracking
- `inference.go`: Leaves model loading, inference and blocking thresholds
- `proxy.go`: HTTP client, backend forwarding, header utilities
- `benchmark.go`: gateway-only and model-only benchmark endpoints
- `helpers.go`: shared conversions
- `features_test.go`: basic regression tests for features, thresholds and session updates

## Functional equivalence

All 37 original function bodies were confirmed to be text-equivalent after `gofmt` during packaging. Existing constants, structs, and global variable definitions were relocated without intentionally changing their values. The files compiled and the tests passed in an isolated local Go build against a temporary Leaves **compile-time stub**. This does not replace testing with your actual Leaves library and final trained model.

No security-policy changes were made. In particular, the pre-existing **fail-open on inference error** path remains for now; this should be reviewed separately, with explicit tests and a policy decision, before describing the system as production hardened. Also, the current benchmark modes are retained unchanged.

## Applying to your clean GitHub copy (Windows PowerShell)

1. Stop `go run .` with Ctrl+C.
2. Back up the current single-file main.go outside the repository.
3. Extract the ZIP and copy its `.go` files into `D:\github-portfolio\api-shield-gateway`, overwriting only `main.go`.
4. Retain existing `go.mod`, `go.sum` and the final STRICT-12 model `.txt` file.
5. In the repository directory, run:

```powershell
go test ./...
go build -o "$env:TEMP\api-shield-modular-verify.exe" .
```

6. Run with `go run .`, **not** `go run main.go` (the application now spans multiple files).
7. Run your previous smoke test for `/__benchmark/model` and compare with the earlier result before publishing.

## Remaining tasks before public GitHub publishing

- Add and verify `.gitignore` and scan for secrets.
- Add a clear README with architecture, documented limitations, and reproducible run instructions.
- Keep thesis-era performance claims distinct from parity-corrected implementation results.
- Consider more systematic tests, including error paths, race testing on supported Windows toolchains, and Python–Go prediction parity.
