# Agent Guidelines for stockfinlens

## Release Checklist

Every new release **must** follow this checklist before building and publishing.

### 1. Version Sync & Bump After Release (Hard Requirement)
The following two files must contain the **exact same version number**:

- `wails.json` → `info.productVersion`
- `frontend/src/Settings.tsx` → `const version`

> **Why?** `wails.json` drives the Windows executable metadata, while `frontend/src/Settings.tsx` drives the version shown in **Settings → About**. If they diverge, users will see the wrong version in the UI.

Both build scripts (`build-release.sh` and `build-windows.ps1`) now enforce this check and will **fail the build** if the versions do not match.

#### Version Bump Rule
**Immediately after a release is published, bump both version numbers to the *next* unreleased version** (e.g., after releasing `v1.3.13`, bump to `1.3.14`).

> **Rationale:** Any intermediate build produced from `main` should identify itself as the *upcoming* version, not the already-released old version. This prevents confusion when sharing test builds or debugging artifacts.

### 2. Build Prerequisites
- Ensure `frontend/dist` is rebuilt from scratch when the frontend code has changed:
  ```bash
  cd frontend && npm run build
  ```
  > Wails `build` may skip the frontend build if `frontend/dist` already exists, which can embed stale code into the binary.

### 3. Required Assets in Release Package
The final release ZIP must include:
- `stockfinlens.exe` (or `.app` on macOS)
- `ml_models/` — ML model files and Python inference scripts
- `scripts/` — Python data-fetching and update scripts

> **Why?** The Go backend looks for Python scripts in the `scripts/` directory relative to the executable at runtime. Missing `scripts/` will break: policy library updates, industry database updates, HK profile/financials fetching, and RIM data fetching.

### 4. CHANGELOG Update
Append the new version section to the top of `CHANGELOG.md` before tagging.

### 5. Tag & Release
1. Commit all changes.
2. Push to `origin/main`.
3. Create and push Git tag (e.g., `v1.3.13`).
4. Build the release package(s).
5. Create GitHub Release with the tag and upload the ZIP asset(s).
6. Copy the full `CHANGELOG.md` section into the Release notes body.
