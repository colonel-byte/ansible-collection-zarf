# dependabot validate

Validates [`.github/dependabot.yaml`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.github/dependabot.yaml) against Dependabot's schema, and posts the result as a sticky comment on the pull request.

It is path-filtered: it runs only when the pull request touches `.github/dependabot.yaml` or the workflow itself.

This exists because GitHub reports a malformed Dependabot configuration on the repository's Insights tab and nowhere a pull request author would look. A typo that stops an ecosystem from being updated is otherwise silent -- the updates simply never arrive.

The comment step runs with `if: always()`, so a failed validation is reported in the comment rather than only in the job log.
