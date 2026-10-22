# Why Dependabot gets a commitlint config of its own

`title_check` in [`commitlint.yaml`](../workflows/commitlint.yaml.md) lints the pull request title rather than the commits in it, because release-please squash-merges and the title becomes the final commit message. It is also a required status check on `main`.

Dependabot's titles break one rule in that config and cannot be made to stop. This is the record of what was done about it.

## The problem

`@commitlint/config-conventional` sets `header-max-length` to 100. Dependabot writes `build(deps): bump <module> from X to Y`, and it cannot shorten a module path -- there is no template knob that abbreviates it, and the path is the one piece of information the title exists to carry.

Ten of the modules in `go.mod` overflow 100 characters that way. The longest, `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`, reaches 117. With a daily `gomod` schedule, that is a steady stream of pull requests that cannot pass a required check.

Collapsing the `gomod` ecosystem into one group made most of them titles of the form `build(deps): bump the gomod group across 1 directory with N updates`, which fits -- see [`choice-dependabot-vendor-grouping.md`](choice-dependabot-vendor-grouping.md). It did not remove the problem: Dependabot opens a security advisory update outside the group, naming the one module, and that title is as long as it ever was.

## Why not the two easier answers

**Skipping the job for Dependabot** leaves `title_check` with no result at all. A required status check that never reports is not a passing check -- the pull request stays unmergeable, which is the problem it was meant to solve. The same reasoning is why the job runs on `merge_group` and reports success with every step skipped: the queue needs to see a result.

**Turning `header-max-length` off everywhere** gives up the rule on human titles too, and a human title is a title somebody can shorten. The rule is worth keeping where it can be met.

## What is there instead

`.github/commitlint/.commitlintrc.dependabot.yaml` is `.commitlintrc.yaml` with `header-max-length` set to `0`, and the lint step selects it by author:

```sh
config=.commitlintrc.yaml
if [ "${PR_AUTHOR}" = "dependabot[bot]" ]; then
  config=.commitlintrc.dependabot.yaml
fi
```

Every other rule still applies, so a malformed type or subject in a Dependabot title fails exactly the way it would in a human's. The check reports, and the length rule is relaxed for the one author that cannot satisfy it.

`PR_AUTHOR` and `PR_TITLE` reach the script through `env:` rather than being interpolated into the `run:` block, which is the rule [`.github/AGENTS.md`](https://github.com/colonel-byte/ansible-collection-zarf/blob/main/.github/AGENTS.md) states for any `github.event.*` value.

## The cost

Two files that must agree. There is no shared base, because commitlint cannot extend a YAML sibling, so a rule added or changed in `.commitlintrc.yaml` has to be copied into `.commitlintrc.dependabot.yaml` by hand. Both files carry a comment saying so, and nothing in CI checks it -- the configs can drift, and the symptom would be a rule that is enforced on humans and quietly not on Dependabot.

If commitlint ever gains sibling extension, collapsing these two into a base plus one override is the fix.
