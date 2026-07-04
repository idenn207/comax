# GitHub Actions 통합 (moved)

The user-facing GitHub Actions guide is now canonical on the docs site.

- **Source of truth**: [`website/content/docs/github-actions.mdx`](../website/content/docs/github-actions.mdx)
- **Rendered**: `/docs/github-actions` on the deployed docs site (M6)

Edit the MDX under `website/content/docs`, not this file. The action contract
itself lives in [`action.yml`](../action.yml); the docs `check-docs-drift.mjs`
gate asserts the MDX matches that source.
