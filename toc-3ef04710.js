// Populate the sidebar
//
// This is a script, and not included directly in the page, to control the total size of the book.
// The TOC contains an entry for each page, so if each page includes a copy of the TOC,
// the total size of the page becomes O(n**2).
class MDBookSidebarScrollbox extends HTMLElement {
    constructor() {
        super();
    }
    connectedCallback() {
        this.innerHTML = '<ol class="chapter"><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="index.html">readme</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="security.html">security</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ZEP-finds.html">ZEP findings</a></span></li><li class="chapter-item expanded "><li class="spacer"></li></li><li class="chapter-item expanded "><li class="part-title">Guides</li></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="guides/package-prerequisites.html">package-prerequisites</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="guides/prerequisites-package-id-groups.html">prerequisites-package-id-groups</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="guides/zarf-ansible-module.html">zarf-ansible-module</a></span></li><li class="chapter-item expanded "><li class="spacer"></li></li><li class="chapter-item expanded "><li class="part-title">Ansible</li></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/collection.html">zarf collection</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/modules.html">zarf modules</a></span><ol class="section"><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_cluster_info.html">cluster_info</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_init.html">init</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_kubectl.html">kubectl</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_deploy.html">package_deploy</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_documentation.html">package_documentation</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_info.html">package_info</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_inspect.html">package_inspect</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_mirror_resources.html">package_mirror_resources</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_pull.html">package_pull</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_package_remove.html">package_remove</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_state_info.html">state_info</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/module_update_creds.html">update_creds</a></span></li></ol><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/roles.html">zarf roles</a></span><ol class="section"><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/role_deploy_packages.html">deploy_packages</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/role_package_inspect.html">package_inspect</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/role_packages.html">packages</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/role_prerequisites.html">prerequisites</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="ansible/role_state.html">state</a></span></li></ol><li class="chapter-item expanded "><li class="spacer"></li></li><li class="chapter-item expanded "><li class="part-title">Development</li></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="dev/e2e-tests.html">e2e-tests</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="dev/mage.html">mage</a></span></li><li class="chapter-item expanded "><li class="spacer"></li></li><li class="chapter-item expanded "><li class="part-title">Workflows</li></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/check-go-mod.yaml.html">check-go-mod.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/codeql.yaml.html">codeql.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/commitlint.yaml.html">commitlint.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/dependabot-validate.yaml.html">dependabot-validate.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/deploy-book.yaml.html">deploy-book.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/e2e-cluster.yaml.html">e2e-cluster.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/e2e.yaml.html">e2e.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/pre-commit.yaml.html">pre-commit.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/release-please.yaml.html">release-please.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/release.yaml.html">release.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/scan-lint.yaml.html">scan-lint.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/scorecard.yaml.html">scorecard.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/test-build-containers.yaml.html">test-build-containers.yaml</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="workflows/unit-tests.yaml.html">unit-tests.yaml</a></span></li><li class="chapter-item expanded "><li class="spacer"></li></li><li class="chapter-item expanded "><li class="part-title">Agent</li></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/ai-usage.html">ai-usage</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-dependabot-title-length.html">choice-dependabot-title-length</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-dependabot-vendor-grouping.html">choice-dependabot-vendor-grouping</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-deploy-packages-drift-scope.html">choice-deploy-packages-drift-scope</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-fuzz-targets.html">choice-fuzz-targets</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-gitops-engine-key.html">choice-gitops-engine-key</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-package-documentation-module.html">choice-package-documentation-module</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-prerequisites-package-dag.html">choice-prerequisites-package-dag</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-prerequisites-package-names.html">choice-prerequisites-package-names</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-version-tracks-zarf.html">choice-version-tracks-zarf</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-zarf-ansible-module.html">choice-zarf-ansible-module</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-zarf-collection-removal.html">choice-zarf-collection-removal</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/choice-zarf-info-modules.html">choice-zarf-info-modules</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/todo-deploy-packages-component-selection.html">todo-deploy-packages-component-selection</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/todo-deploy-packages-item-contract.html">todo-deploy-packages-item-contract</a></span></li><li class="chapter-item expanded "><span class="chapter-link-wrapper"><a href="agent/todo-roles-are-single-host.html">todo-roles-are-single-host</a></span></li></ol>';
        // Set the current, active page, and reveal it if it's hidden
        let current_page = document.location.href.toString().split('#')[0].split('?')[0];
        if (current_page.endsWith('/')) {
            current_page += 'index.html';
        }
        const links = Array.prototype.slice.call(this.querySelectorAll('a'));
        const l = links.length;
        for (let i = 0; i < l; ++i) {
            const link = links[i];
            const href = link.getAttribute('href');
            if (href && !href.startsWith('#') && !/^(?:[a-z+]+:)?\/\//.test(href)) {
                link.href = path_to_root + href;
            }
            // The 'index' page is supposed to alias the first chapter in the book.
            // Check both with and without the '.html' suffix to be robust against pretty URLs
            if (link.href.replace(/\.html$/, '') === current_page.replace(/\.html$/, '')
                || i === 0
                && path_to_root === ''
                && current_page.endsWith('/index.html')) {
                link.classList.add('active');
                let parent = link.parentElement;
                while (parent) {
                    if (parent.tagName === 'LI' && parent.classList.contains('chapter-item')) {
                        parent.classList.add('expanded');
                    }
                    parent = parent.parentElement;
                }
            }
        }
        // Track and set sidebar scroll position
        this.addEventListener('click', e => {
            if (e.target.tagName === 'A') {
                const clientRect = e.target.getBoundingClientRect();
                const sidebarRect = this.getBoundingClientRect();
                sessionStorage.setItem('sidebar-scroll-offset', clientRect.top - sidebarRect.top);
            }
        }, { passive: true });
        const sidebarScrollOffset = sessionStorage.getItem('sidebar-scroll-offset');
        sessionStorage.removeItem('sidebar-scroll-offset');
        if (sidebarScrollOffset !== null) {
            // preserve sidebar scroll position when navigating via links within sidebar
            const activeSection = this.querySelector('.active');
            if (activeSection) {
                const clientRect = activeSection.getBoundingClientRect();
                const sidebarRect = this.getBoundingClientRect();
                const currentOffset = clientRect.top - sidebarRect.top;
                this.scrollTop += currentOffset - parseFloat(sidebarScrollOffset);
            }
        } else {
            // scroll sidebar to current active section when navigating via
            // 'next/previous chapter' buttons
            const activeSection = document.querySelector('#mdbook-sidebar .active');
            if (activeSection) {
                activeSection.scrollIntoView({ block: 'center' });
            }
        }
        // Toggle buttons
        const sidebarAnchorToggles = document.querySelectorAll('.chapter-fold-toggle');
        function toggleSection(ev) {
            ev.currentTarget.parentElement.parentElement.classList.toggle('expanded');
        }
        Array.from(sidebarAnchorToggles).forEach(el => {
            el.addEventListener('click', toggleSection);
        });
    }
}
window.customElements.define('mdbook-sidebar-scrollbox', MDBookSidebarScrollbox);

