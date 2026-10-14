# Release Please

Maintains the release pull request: it accumulates conventional commits landed on `main`, writes the changelog, bumps the version, and tags the release when the pull request merges. Runs on every push to `main`, and on demand from the Actions tab.

Configuration lives in [`release-please-config.json`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/release-please-config.json) and the current version in `.release-please-manifest.json`. The `changelog-sections` block decides which commit types appear in the changelog and under what heading; `perf` and `test` are hidden.

`extra-files` is the part that needs keeping current. Release-please bumps `ansible/colonel_byte/zarf/galaxy.yml`, and it will bump a Markdown page the same way -- but only when the page is annotated with `x-release-please-start-version` markers **and** listed in `extra-files`. Both halves are required: an annotated page that is not listed is never read, and a listed page with no annotation is read and left alone. See [`docs/AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/docs/AGENTS.md) for the annotation form and the hazard of a region holding a second dotted-decimal string.

The workflow declares no write permission anywhere. It mints a GitHub App token with `actions/create-github-app-token` and hands that to release-please, which does every write with it, so `GITHUB_TOKEN` is only along for the ride. The app's credentials come from the `RELEASE_PLEASE_GITHUB_APP_ID` and `RELEASE_PLEASE_GITHUB_APP_SECRET` secrets.
