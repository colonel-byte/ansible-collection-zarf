# GitHub Pages

Builds the mdBook and publishes it to the `gh-pages` branch, which is what serves <https://colonel-byte.github.io/ansible-collection-zarf/>. It runs on every push to `main`.

The job runs `mdbook build` over `docs/` and pushes `./book` with `peaceiris/actions-gh-pages`. That push is the only write the workflow makes, so `contents: write` sits on the job rather than at the top of the file; nothing here touches a pull request.

Two things follow from the book being published this way:

- The generated pages have to be committed. The build does not run `generate:document`, so whatever is in `docs/` at the time of the push is what gets published -- a generated page that was not regenerated is published stale.
- A renamed or moved page breaks whatever links to it from outside the repository, and mdBook does not notice: the old URL simply 404s. A move comes with an entry in `[output.html.redirect]` in `book.toml`. See [`docs/AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/docs/AGENTS.md).
