# PR Title Check

Lints the pull request title against [commitlint](https://commitlint.js.org/), using the config in `.github/commitlint/`. It runs on a pull request opened, reopened, synchronized, edited or milestoned, and on the merge queue.

It lints the **title**, not the individual commits: release-please squash-merges, so the pull request title becomes the final commit message and is the thing that has to parse as a conventional commit. `wagoid/commitlint-github-action` lints commit messages instead, which is why it is not a drop-in replacement for the step here.

`milestoned` is in the trigger list as a workaround for release-please not triggering pull request workflows -- adding its pull request to a milestone is what gets the check to run.

Under `merge_group` there is no pull request to read, so every step is skipped and the job reports success. That is deliberate: `title_check` is a required status check, so the merge queue has to see a result on the `merge_group` ref too, and the title was already linted on the pull request that got queued.

Dependabot gets `.commitlintrc.dependabot.yaml`, the same config with `header-max-length` turned off. It cannot shorten a module path, and ten of the modules in `go.mod` overflow a 100-character title. Skipping the job for Dependabot instead would leave `title_check` without a result, and that is a required check on `main`. See [choice-dependabot-title-length](../agent/choice-dependabot-title-length.md).
