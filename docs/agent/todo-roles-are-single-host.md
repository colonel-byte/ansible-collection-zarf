# TODO: the roles say "once" fifteen times and nothing tests it with two hosts

Every role in this collection delegates its work to one node and marks the tasks `run_once: true`. Nothing in the repository runs any of them against a play with more than one host, so the whole design is unexercised: every playbook under `test/e2e/noncluster/testdata/` and `test/e2e/cluster/testdata/`, and every example under `ansible/examples/playbooks/`, is `hosts: localhost` with `connection: local`.

This is not a hypothetical shape. The collection's own scope statement in [`ansible/colonel_byte/zarf/AGENTS.md`](../../ansible/colonel_byte/zarf/AGENTS.md) says zarf talks to one cluster from the node it runs on, and a management-node play that targets a fleet of clusters -- one host per cluster -- is the obvious way an operator would use it. That play has never been run.

## What is uncertain

`colonel_byte.zarf.deploy_packages` is the sharpest case. `tasks/main.yml` loops `include_tasks: deploy_package.yml` with **no** `run_once`, while almost every task inside the included file has one. So the loop is entered once per host and the inner tasks are what stop zarf running once per host.

Under the `linear` strategy the hosts move in lockstep and a `run_once` task's facts propagate to the rest of the play, so this is expected to work. Under `strategy: free` the hosts desynchronise, and then `_components_already_present` -- a fact, set `run_once`, read by a task that is itself `run_once` -- is read by whichever host arrived first against whichever item's value was written last. The same question applies to the three `include_tasks` in the `prerequisites` role's `main.yml` and the one in its `check_versions.yml`.

The milder version of the same doubt is cosmetic: if `run_once` inside a per-host include fires once per expansion rather than once per play, `zarf_prerequisites_checked` gains a copy of every message per host and the final assert's message is N times as long. Wrong-looking rather than wrong, since what matters is whether the unmet list is empty.

## Why it is written down rather than settled

It needs a two-host play to settle, and a conclusion drawn from one is only as good as the strategy it ran under -- `linear` passing says nothing about `free`. That is a fixture plus a decision about which strategies the collection claims to support, and nothing claims any today.

There is a second reason to be careful: `run_once: true` on the `include_tasks` itself would be smaller than fifteen inner ones and is probably more robust, but it is a behaviour change to how facts propagate, and making it without a multi-host test is trading an untested design for a different untested design.

## What a real fix is

1. **A fixture with two hosts.** The noncluster suite can do it with an inline inventory of two aliases both resolving to localhost, run once under `linear` and once under `strategy: free`. That is cheap -- the stub serves both -- and it is what turns every statement above into a fact.
2. **Then decide.** Either move `run_once` up to the includes, or state the supported strategies in `meta/argument_specs.yml` and leave the tasks alone. Both are small once the fixture exists; neither is safe before it.

Until then, treat the roles as single-host. `meta/main.yml` claims `min_ansible_version: "2.15.0"`, which widens the uncertainty rather than narrowing it: ansible-core 2.19 rewrote the templating engine, and every templating behaviour this repository has verified was verified on 2.20.
