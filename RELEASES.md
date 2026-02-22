# Releasing TideTerm

This repository is a fork of Wave Terminal. TideTerm does **not** use the upstream release infrastructure (S3 buckets, bots, etc.) by default.

## Build artifacts locally

1. Install dependencies and initialize the repo:

```sh
task init
```

2. Build and package production artifacts:

```sh
task package
```

Artifacts will be written to `make/`.

If you're on Linux ARM64 and you prefer to use a system-installed `fpm`, you may need:

```sh
USE_SYSTEM_FPM=1 task package
```

## Publish on GitHub

1. Create a new tag (choose your own versioning scheme).
2. Create a GitHub Release in `sanshao85/tideterm`.
3. Upload the build artifacts from `make/` to the release.

## Pre-release checklist (fork + compliance)

Before publishing a release, verify the following:

- Version/identity is updated consistently in `package.json` (version/product metadata) and release tag name.
- Fork change summary is current in `MODIFICATIONS.md` (major functional deltas vs upstream).
- Release-facing fork diff is current in `RELEASE_NOTES_FORK_DIFF.md` (includes copyable CN/EN release bullets).
- README entry points are present and valid in `README.md` and `README.zh-CN.md` (links to fork diff/release notes docs).
- License/notice files are preserved and still correct (`LICENSE`, `NOTICE`).
- Default behavior changes are documented when relevant (for example telemetry/autoupdate defaults, config path namespace, migration notes).
- If upstream comparison baseline changes, update the upstream commit reference/date in `MODIFICATIONS.md` and `RELEASE_NOTES_FORK_DIFF.md`.

Optional but recommended:

- Include a short "What's Changed" section in the GitHub Release body using the summary in `RELEASE_NOTES_FORK_DIFF.md`.
- Add upgrade/migration notes when changing app identifiers, data directories, or remote helper paths.

## Automatic updates

TideTerm ships with automatic updates **disabled by default**. If you choose to enable them, you will need to:

- Provide a stable update feed (typically GitHub Releases via `electron-updater`)
- Configure signing/notarization (macOS) and signing (Windows) if you distribute signed builds

## Notes for forks

If you change identifiers (app name/appId/data paths), treat it as a breaking change for existing installs and consider documenting migration steps in release notes.
