# Documentation Deployment

## Overview

MkDocs source lives in `assets/mkdocs/`. Built HTML is **never committed** to
the repository. GitHub Actions builds the site and deploys it directly to
GitHub Pages using the modern Actions-based deployment (no `gh-pages` branch,
no committed `docs/` folder).

## Local preview

```
just docs-serve
```

Opens a live-reload server at <http://localhost:7000>. Changes to files in
`assets/mkdocs/` are reflected immediately without restarting the server.

No separate install step is needed — `docs-serve` syncs Python dependencies
via `uv` automatically.

## How CI/CD works

The workflow at `.github/workflows/docs.yml` runs on every push to `main` when
files under `assets/` change. It uses two GitHub Actions jobs:

```
build  →  upload Pages artifact (site/)
deploy →  publish artifact to GitHub Pages
```

No HTML is committed back to the repository. GitHub Pages serves the artifact
uploaded by the workflow.

### Required one-time repository setting

In the GitHub repository go to **Settings → Pages → Build and deployment →
Source** and select **GitHub Actions**. This only needs to be done once.

## Directory layout

```
assets/
  mkdocs.yml          # MkDocs configuration (site_dir: ../site)
  pyproject.toml      # Python dependencies (mkdocs-material, etc.)
  uv.lock             # Locked dependency versions
  mkdocs/             # Markdown source files
    index.md
    getting-started.md
    changelog.md
    user-guide/
    developer/
    stylesheets/
```

`site/` is the build output directory. It is listed in `.gitignore` and must
not be committed.

## Why site_dir is `../site` not `../docs`

Using `site/` (the MkDocs default) makes it immediately clear that the folder
is generated output. The old `docs/` convention conflicted with GitHub's
"Deploy from a branch → /docs" mode and led to built HTML being tracked in git.
