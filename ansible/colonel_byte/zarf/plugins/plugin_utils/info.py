# Copyright 2026 colonel-byte
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""The shared plumbing of the read-only zarf modules.

Everything the two converging modules do lives in Go, in internal/zarfmod, and the Python beside
them is delivery and display only. See plugin_utils/projection.py, which says so, and
docs/agent/choice-zarf-ansible-module.md, which says why.

The read-only modules are deliberately not built that way, and docs/agent/choice-zarf-info-modules.md
is the record of that decision. What they share is here rather than copied three times: resolving
the zarf binary, rendering the environment the command runs in, running it, and turning a non-zero
exit into one failed task. What each one does not share -- which zarf command it runs, and what its
output means -- is the two methods a subclass fills in.

A subclass implements ``zarf_argv`` by returning its module-level ``command_parts``, which is a
plain function of the task's parameters so that internal/zarfmod/infoplugins_test.go can read the
flags it renders out of the source. It implements ``interpret`` to turn the command's stdout into
the keys the module returns.

One module asks the cluster more than one question, and for it there is a second shape. A subclass
that sets ``MULTI = True`` implements ``zarf_argvs`` -- returning a list of (name, argv) pairs --
and ``interpret_outputs``, which receives the stdout of each one keyed by that name. It exists for
``zarf_cluster_info``: the Kubernetes version, the CRDs, the StorageClasses and the Deployments are
four kubectl queries and no amount of flags makes them one, while splitting them across four
modules would make a prerequisite check four tasks that can each half-fail. The commands run in the
order given and the first non-zero exit fails the task, naming which one it was.
"""

from __future__ import absolute_import, division, print_function

__metaclass__ = type

import os
import shlex
import shutil

from ansible.plugins.action import ActionBase

# The parameter naming the zarf to run. Named here because the failure message below names it, and
# a message that names the wrong parameter is worse than one that names none.
BINARY = "zarf_binary"

# The parameter naming the kubeconfig. Zarf has no --kubeconfig flag, so it is passed as KUBECONFIG
# in zarf's environment, the same way the converging modules pass it.
KUBECONFIG = "kubeconfig"

# The name the output of a single-command module is collected under, so that run() has one code
# path rather than a branch per shape. No subclass sees it: a module with one command gets its
# stdout handed to interpret() as a string, the way it did before there were two shapes.
SOLE = "sole"


class ZarfInfoActionBase(ActionBase):
    """Base for the read-only zarf module action plugins.

    A subclass sets ``REQUIRED`` to the parameters that have no useful default, and implements
    ``zarf_argv`` and ``interpret``.
    """

    # Nothing is copied to a managed node: zarf runs on the node the task is delegated to and
    # reaches the cluster through a kubeconfig.
    TRANSFERS_FILES = False

    REQUIRED = ()

    # MULTI says which pair of methods the subclass implements: zarf_argv and interpret, or
    # zarf_argvs and interpret_outputs. It is an explicit declaration rather than a check for
    # whether a method was overridden, because the failure mode of guessing is a module that
    # silently runs one command and reports on it as though it had run four.
    MULTI = False

    def run(self, tmp=None, task_vars=None):
        if task_vars is None:
            task_vars = {}

        result = super(ZarfInfoActionBase, self).run(tmp, task_vars)
        del tmp  # deprecated, and unused: nothing is transferred

        args = dict(self._task.args)

        for name in self.REQUIRED:
            if not args.get(name):
                result["failed"] = True
                result["msg"] = "missing required argument: %s" % name
                return result

        binary = args.get(BINARY) or "zarf"
        resolved = binary if os.path.sep in binary else shutil.which(binary)
        if not resolved:
            # The alternative is to run the unresolved name and let the shell answer, which reports
            # "command not found" wrapped in this module's own failure message and names no
            # parameter the operator could set. See projection.py, which fails the same way.
            result["failed"] = True
            result["msg"] = (
                "%s is not on the path of the node this task runs on. Install zarf, or set the "
                "%s parameter to its path." % (binary, BINARY)
            )
            return result

        params = dict(args, **{BINARY: resolved})
        kubeconfig = args.get(KUBECONFIG)

        outputs = {}
        for name, argv in self._commands(params):
            rc, stdout, stderr = self._run_zarf(argv, kubeconfig)
            if rc != 0:
                result["failed"] = True
                result["msg"] = "%s failed: %s" % (
                    " ".join(argv),
                    stderr.strip() or stdout.strip() or "zarf exited %d with no output" % rc,
                )
                result["rc"] = rc
                result["stderr"] = stderr
                # Which question went unanswered, for a module that asks several. A message naming
                # only the command line leaves the reader to work out which of four kubectl
                # queries that was.
                if self.MULTI:
                    result["query"] = name
                return result
            outputs[name] = stdout

        # A read-only module changes nothing, which is also why no subclass answers check mode:
        # running during a check-mode play is the honest thing for it to do.
        result["changed"] = False
        if self.MULTI:
            self.interpret_outputs(result, outputs, args)
        else:
            self.interpret(result, outputs[SOLE], args)
        return result

    def _commands(self, params):
        """Return the commands to run, as (name, argv) pairs, whichever shape the subclass uses."""
        if self.MULTI:
            return list(self.zarf_argvs(params))
        return [(SOLE, self.zarf_argv(params))]

    def _run_zarf(self, argv, kubeconfig):
        """Run one zarf command and return its exit status and streams.

        The environment is part of the command rather than passed separately, because
        ``_low_level_execute_command()`` takes a command and not an environment. Every word is
        quoted: a path holding a space is an operator's choice, not an injection.
        """
        parts = []
        if kubeconfig:
            parts.extend(["env", "KUBECONFIG=%s" % shlex.quote(str(kubeconfig))])
        parts.extend(shlex.quote(str(word)) for word in argv)

        res = self._low_level_execute_command(" ".join(parts), executable="/bin/sh")
        return (
            res.get("rc", 0),
            res.get("stdout", "") or "",
            res.get("stderr", "") or "",
        )

    def zarf_argv(self, params):
        """Return the zarf command line to run, unquoted and without its environment."""
        raise NotImplementedError

    def interpret(self, result, stdout, params):
        """Put what the command printed into the module's result."""
        raise NotImplementedError

    def zarf_argvs(self, params):
        """Return the zarf command lines to run, as (name, argv) pairs. For MULTI subclasses."""
        raise NotImplementedError

    def interpret_outputs(self, result, outputs, params):
        """Put what the commands printed into the module's result, keyed by name. For MULTI."""
        raise NotImplementedError
