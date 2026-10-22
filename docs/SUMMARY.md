# Index

[readme](index.md)
[security](security.md)
[ZEP findings](ZEP-finds.md)

-----------

# Guides

- [package-prerequisites](guides/package-prerequisites.md)
- [prerequisites-package-id-groups](guides/prerequisites-package-id-groups.md)
- [zarf-ansible-module](guides/zarf-ansible-module.md)


-----------

# Ansible

- [zarf collection](ansible/collection.md)
- [zarf modules](ansible/modules.md)
  - [cluster_info](ansible/module_cluster_info.md)
  - [init](ansible/module_init.md)
  - [kubectl](ansible/module_kubectl.md)
  - [package_deploy](ansible/module_package_deploy.md)
  - [package_documentation](ansible/module_package_documentation.md)
  - [package_info](ansible/module_package_info.md)
  - [package_inspect](ansible/module_package_inspect.md)
  - [package_mirror_resources](ansible/module_package_mirror_resources.md)
  - [package_pull](ansible/module_package_pull.md)
  - [package_remove](ansible/module_package_remove.md)
  - [state_info](ansible/module_state_info.md)
  - [update_creds](ansible/module_update_creds.md)
- [zarf roles](ansible/roles.md)
  - [deploy_packages](ansible/role_deploy_packages.md)
  - [package_inspect](ansible/role_package_inspect.md)
  - [packages](ansible/role_packages.md)
  - [prerequisites](ansible/role_prerequisites.md)
  - [state](ansible/role_state.md)


-----------

# Development

<!-- Excluded from the print page (print.html) by docs/css/print.css. -->

- [e2e-tests](dev/e2e-tests.md)
- [mage](dev/mage.md)


-----------

# Workflows

<!-- Excluded from the print page (print.html) by docs/css/print.css. -->

- [check-go-mod.yaml](workflows/check-go-mod.yaml.md)
- [codeql.yaml](workflows/codeql.yaml.md)
- [commitlint.yaml](workflows/commitlint.yaml.md)
- [dependabot-validate.yaml](workflows/dependabot-validate.yaml.md)
- [deploy-book.yaml](workflows/deploy-book.yaml.md)
- [e2e-cluster.yaml](workflows/e2e-cluster.yaml.md)
- [e2e.yaml](workflows/e2e.yaml.md)
- [pre-commit.yaml](workflows/pre-commit.yaml.md)
- [release-please.yaml](workflows/release-please.yaml.md)
- [release.yaml](workflows/release.yaml.md)
- [scan-lint.yaml](workflows/scan-lint.yaml.md)
- [scorecard.yaml](workflows/scorecard.yaml.md)
- [test-build-containers.yaml](workflows/test-build-containers.yaml.md)
- [unit-tests.yaml](workflows/unit-tests.yaml.md)


-----------

# Agent

<!-- Excluded from the print page (print.html) by docs/css/print.css. -->

- [ai-usage](agent/ai-usage.md)
- [choice-dependabot-title-length](agent/choice-dependabot-title-length.md)
- [choice-dependabot-vendor-grouping](agent/choice-dependabot-vendor-grouping.md)
- [choice-deploy-packages-drift-scope](agent/choice-deploy-packages-drift-scope.md)
- [choice-fuzz-targets](agent/choice-fuzz-targets.md)
- [choice-gitops-engine-key](agent/choice-gitops-engine-key.md)
- [choice-package-documentation-module](agent/choice-package-documentation-module.md)
- [choice-version-tracks-zarf](agent/choice-version-tracks-zarf.md)
- [choice-zarf-ansible-module](agent/choice-zarf-ansible-module.md)
- [choice-zarf-collection-removal](agent/choice-zarf-collection-removal.md)
- [choice-zarf-info-modules](agent/choice-zarf-info-modules.md)
- [todo-deploy-packages-component-selection](agent/todo-deploy-packages-component-selection.md)
- [todo-deploy-packages-item-contract](agent/todo-deploy-packages-item-contract.md)
- [todo-roles-are-single-host](agent/todo-roles-are-single-host.md)
