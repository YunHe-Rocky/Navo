# Navo Remediation Baseline

Date: 2026-08-01 (Asia/Shanghai)

## Toolchain

- Go: `go1.26.4 windows/386`
- Frontend package manager: npm using `navo_app/package-lock.json`

## Go baseline

- Command: `powershell.exe -ExecutionPolicy Bypass -File scripts/test.ps1`
- Result: passed
- Coverage: all Go packages plus `go vet` as defined by the repository script

## Frontend baseline

- `npm ci`: passed, 45 packages installed, 0 vulnerabilities reported
- `npm run typecheck`: passed
- `npm run test`: failed because `package.json` has no `test` script
- `npm run build`: passed; Vite transformed 18 modules and produced the production bundle

## Classification

The missing frontend test command is an existing baseline/release-gate failure, not introduced by the four-Guide remediation. It must be fixed before the formal package gate can satisfy the Full Remediation Guide.
