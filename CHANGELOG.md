# Changelog

## [0.89.0](https://github.com/colonel-byte/ansible-collection-zarf/compare/v0.88.0...v0.89.0) (2026-11-01)


### ⚠ BREAKING CHANGES

* **deps:** update zarf to v0.89.0 and name the injector image proxy mode requires ([#169](https://github.com/colonel-byte/ansible-collection-zarf/issues/169))

### Features

* **deploy_packages:** deploy a stack of packages with per-component drift detection ([#149](https://github.com/colonel-byte/ansible-collection-zarf/issues/149)) ([79d0144](https://github.com/colonel-byte/ansible-collection-zarf/commit/79d0144eaa0bec8098696daac2e84ca7ac0e7e94))
* **prerequisites:** name a required package by any of several names ([#148](https://github.com/colonel-byte/ansible-collection-zarf/issues/148)) ([7df5395](https://github.com/colonel-byte/ansible-collection-zarf/commit/7df53956c11e580360956c5b70f017563fd23239))


### Bug Fixes

* **zarfmod:** stop losing a multi-line set-variables value ([#164](https://github.com/colonel-byte/ansible-collection-zarf/issues/164)) ([f275627](https://github.com/colonel-byte/ansible-collection-zarf/commit/f275627a32d9446599b96f94ec7ad621f2b05c61))


### Documentation

* record the ZEP findings and the decisions behind the role's limits ([#151](https://github.com/colonel-byte/ansible-collection-zarf/issues/151)) ([1a28e5f](https://github.com/colonel-byte/ansible-collection-zarf/commit/1a28e5fc74822c687613880f191caa78f7e511a1))


### Miscellaneous

* **deps:** update zarf to v0.89.0 and name the injector image proxy mode requires ([#169](https://github.com/colonel-byte/ansible-collection-zarf/issues/169)) ([970815e](https://github.com/colonel-byte/ansible-collection-zarf/commit/970815ef5e56ba0de5f2785574bcf606b83826e1))


### Build

* **deps:** Bump almalinux/10-base from `ed618ca` to `f72ef25` in /containers/ansible in the containers group ([#162](https://github.com/colonel-byte/ansible-collection-zarf/issues/162)) ([7d66dc6](https://github.com/colonel-byte/ansible-collection-zarf/commit/7d66dc625d6f7f8ae825f000f7e60689a02c0ee7))
* **deps:** Bump github.com/letsencrypt/boulder from 0.20261020.1 to 0.20261027.0 ([#168](https://github.com/colonel-byte/ansible-collection-zarf/issues/168)) ([989c2fa](https://github.com/colonel-byte/ansible-collection-zarf/commit/989c2fab36e75c6702ae17b657282830e16983f5))
* **deps:** Bump https://github.com/astral-sh/ruff-pre-commit from v0.17.2 to 0.17.3 ([#167](https://github.com/colonel-byte/ansible-collection-zarf/issues/167)) ([33252f0](https://github.com/colonel-byte/ansible-collection-zarf/commit/33252f02acd8f5e4c2de846e873cd96da697c90e))
* **deps:** Bump https://github.com/python-jsonschema/check-jsonschema from 0.38.2 to 0.39.0 ([#166](https://github.com/colonel-byte/ansible-collection-zarf/issues/166)) ([36e79b1](https://github.com/colonel-byte/ansible-collection-zarf/commit/36e79b10ebc0b7bee8a9fdca8701ba869451abce))
* **deps:** Bump zarf-dev/zarf/agent from v0.88.0 to v0.89.0 in /containers/ansible in the containers group ([#165](https://github.com/colonel-byte/ansible-collection-zarf/issues/165)) ([ea38740](https://github.com/colonel-byte/ansible-collection-zarf/commit/ea38740bec50c10af7270b5d2f37df158ff6317c))

## [0.88.0](https://github.com/colonel-byte/ansible-collection-zarf/compare/v0.87.0...v0.88.0) (2026-10-26)


### ⚠ BREAKING CHANGES

* **e2e:** cover every acting module, and fix what that found ([#66](https://github.com/colonel-byte/ansible-collection-zarf/issues/66))

### Features

* **api:** add the PackagePrerequisites format and its generated schema ([#50](https://github.com/colonel-byte/ansible-collection-zarf/issues/50)) ([607cd85](https://github.com/colonel-byte/ansible-collection-zarf/commit/607cd8548a7622f0f7d8dc59956f1fb73b414774))
* **collection:** add the prerequisites role ([#54](https://github.com/colonel-byte/ansible-collection-zarf/issues/54)) ([de646cc](https://github.com/colonel-byte/ansible-collection-zarf/commit/de646cc06b2b2ad4f9f6a678ab6d07c205ea159a))
* **collection:** add the zarf_cluster_info module ([#53](https://github.com/colonel-byte/ansible-collection-zarf/issues/53)) ([6060919](https://github.com/colonel-byte/ansible-collection-zarf/commit/6060919dd19d25d0c84a0926fa0ed53d5e31e582))
* **collection:** add the zarf_package_documentation module ([#52](https://github.com/colonel-byte/ansible-collection-zarf/issues/52)) ([3ec8f41](https://github.com/colonel-byte/ansible-collection-zarf/commit/3ec8f415655253cb38a0ba29c67218ca52048c32))
* **examples:** add the reference packages and their vendoring targets ([#51](https://github.com/colonel-byte/ansible-collection-zarf/issues/51)) ([549b621](https://github.com/colonel-byte/ansible-collection-zarf/commit/549b621baf58993ae71b0a0dacf4979732f23b88))
* **modules:** add the zarf_package_pull and zarf_package_remove modules ([#64](https://github.com/colonel-byte/ansible-collection-zarf/issues/64)) ([51288a1](https://github.com/colonel-byte/ansible-collection-zarf/commit/51288a15e506ffa1046899815d805454e5d66d1f))
* **modules:** add the zarf_update_creds and zarf_package_mirror_resources modules ([#65](https://github.com/colonel-byte/ansible-collection-zarf/issues/65)) ([1c3c679](https://github.com/colonel-byte/ansible-collection-zarf/commit/1c3c679b7f33ad4bbf110cc9a353593d12889410))
* **modules:** dispatch one binary on argv[0] and add zarf_kubectl ([#63](https://github.com/colonel-byte/ansible-collection-zarf/issues/63)) ([945fa6b](https://github.com/colonel-byte/ansible-collection-zarf/commit/945fa6bb490dd25dc4d4cc6de015f823f3eaa9d4))
* **modules:** execute zarf's init and package deploy in-process ([#62](https://github.com/colonel-byte/ansible-collection-zarf/issues/62)) ([242f821](https://github.com/colonel-byte/ansible-collection-zarf/commit/242f821e991ba4967ef7a7f27b4aa63b90a0fa35))
* **package_inspect:** report the package's documentation mapping ([#49](https://github.com/colonel-byte/ansible-collection-zarf/issues/49)) ([7c6606b](https://github.com/colonel-byte/ansible-collection-zarf/commit/7c6606b8704c6ed8ab7fb7cb0617ad3854043ccb))


### Bug Fixes

* **ci:** build with GOTMPDIR off the quota'd /tmp ([#154](https://github.com/colonel-byte/ansible-collection-zarf/issues/154)) ([161f1c5](https://github.com/colonel-byte/ansible-collection-zarf/commit/161f1c5718276a249a71280c0f3c77297de8e120))
* **state_info:** withhold credentials under names zarf does not write yet ([#47](https://github.com/colonel-byte/ansible-collection-zarf/issues/47)) ([dc48fb0](https://github.com/colonel-byte/ansible-collection-zarf/commit/dc48fb019ed9fd58fe268a5482744577ea57e18f))
* **zarfmod:** let a later zarf init introduce the git server ([#158](https://github.com/colonel-byte/ansible-collection-zarf/issues/158)) ([9053ec1](https://github.com/colonel-byte/ansible-collection-zarf/commit/9053ec109e80d62de73ed94a934e1b1712d2f8fc))


### Refactoring

* **zarfmod:** share the parameters both wrappers take ([#45](https://github.com/colonel-byte/ansible-collection-zarf/issues/45)) ([1cd1e7b](https://github.com/colonel-byte/ansible-collection-zarf/commit/1cd1e7bf66ef52a2735e9fdfe0fcf73322e7da88))


### Documentation

* **agents:** record the build/tmp quirk and the zarfmod test naming ([#48](https://github.com/colonel-byte/ansible-collection-zarf/issues/48)) ([b2380b6](https://github.com/colonel-byte/ansible-collection-zarf/commit/b2380b6451a4f88cd9ac4f37dcde10ce1f48c7fe))
* pad markdown tables with a pre-commit hook ([#153](https://github.com/colonel-byte/ansible-collection-zarf/issues/153)) ([60905a2](https://github.com/colonel-byte/ansible-collection-zarf/commit/60905a221dd603451247f8f08214a09f8353dc04))
* **skills:** add the zarf package and prerequisites authoring skills ([#55](https://github.com/colonel-byte/ansible-collection-zarf/issues/55)) ([b7a64aa](https://github.com/colonel-byte/ansible-collection-zarf/commit/b7a64aa4d3d064b14c37efd9b1a7439612d7ee46))


### Tests

* **e2e:** cover every acting module, and fix what that found ([#66](https://github.com/colonel-byte/ansible-collection-zarf/issues/66)) ([2a2b41e](https://github.com/colonel-byte/ansible-collection-zarf/commit/2a2b41e4de3aac43e372a764370d705f50e24fb8))


### CI/CD

* **dependabot:** add groups for gomod ([#121](https://github.com/colonel-byte/ansible-collection-zarf/issues/121)) ([40d06c7](https://github.com/colonel-byte/ansible-collection-zarf/commit/40d06c776c4068c9c571198485f65ab3a40310af))
* **e2e:** install zarf and keep TMPDIR off the repository path ([#44](https://github.com/colonel-byte/ansible-collection-zarf/issues/44)) ([8da6db3](https://github.com/colonel-byte/ansible-collection-zarf/commit/8da6db3fa2daeec395ec7bde68b1d1b5574410bc))
* **fuzz:** add fuzzing to golang ([#38](https://github.com/colonel-byte/ansible-collection-zarf/issues/38)) ([39cff2a](https://github.com/colonel-byte/ansible-collection-zarf/commit/39cff2a3f115c61f9ec27df132b91e2a053e3113))


### Build

* **deps:** Bump atomicgo.dev/keyboard from 0.2.9 to 0.2.10 ([#104](https://github.com/colonel-byte/ansible-collection-zarf/issues/104)) ([5a6ecf9](https://github.com/colonel-byte/ansible-collection-zarf/commit/5a6ecf92814fe051b699f9ef11b97f036669fd96))
* **deps:** Bump buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go from 1.36.11-20260415201107-50325440f8f2.1 to 1.36.12-20260825204119-511051f7f437.2 ([#84](https://github.com/colonel-byte/ansible-collection-zarf/issues/84)) ([97c5c69](https://github.com/colonel-byte/ansible-collection-zarf/commit/97c5c69711dd4605b031e34cd53687a427673af1))
* **deps:** Bump cel.dev/expr from 0.25.2 to 0.25.3 ([#83](https://github.com/colonel-byte/ansible-collection-zarf/issues/83)) ([4d96dc6](https://github.com/colonel-byte/ansible-collection-zarf/commit/4d96dc61d3cde90208c7648dc7d335da73eacd12))
* **deps:** Bump github.com/alibabacloud-go/alibabacloud-gateway-spi from 0.0.4 to 0.0.5 ([#110](https://github.com/colonel-byte/ansible-collection-zarf/issues/110)) ([0b9c02c](https://github.com/colonel-byte/ansible-collection-zarf/commit/0b9c02c1f2ba673d3a210c722c2bcbd4e38f99e4))
* **deps:** Bump github.com/alibabacloud-go/debug from 1.0.0 to 1.0.1 ([#88](https://github.com/colonel-byte/ansible-collection-zarf/issues/88)) ([01d298d](https://github.com/colonel-byte/ansible-collection-zarf/commit/01d298d63c523b776f6eafa1f4a3abc1ae9eef75))
* **deps:** Bump github.com/alibabacloud-go/openapi-util from 0.1.0 to 0.1.2 ([#144](https://github.com/colonel-byte/ansible-collection-zarf/issues/144)) ([af40e79](https://github.com/colonel-byte/ansible-collection-zarf/commit/af40e790fc18b461b78960a14ac2711d55ef30e1))
* **deps:** Bump github.com/aliyun/credentials-go from 1.4.7 to 1.4.13 ([#115](https://github.com/colonel-byte/ansible-collection-zarf/issues/115)) ([79e0416](https://github.com/colonel-byte/ansible-collection-zarf/commit/79e04160a74ae4683d8a8d196b82d3714b422290))
* **deps:** Bump github.com/anchore/bubbly from 0.2.1 to 0.2.2 ([#118](https://github.com/colonel-byte/ansible-collection-zarf/issues/118)) ([de1a669](https://github.com/colonel-byte/ansible-collection-zarf/commit/de1a6694f2e3955faadb2abf63a4638e3b7d7158))
* **deps:** Bump github.com/anchore/go-rpmdb from 0.2.0 to 0.2.2 ([#139](https://github.com/colonel-byte/ansible-collection-zarf/issues/139)) ([82ad45e](https://github.com/colonel-byte/ansible-collection-zarf/commit/82ad45ef4e2c10f0dcb0d35b508137cfaf887803))
* **deps:** Bump github.com/anchore/go-sync from 0.1.1 to 0.1.2 ([#113](https://github.com/colonel-byte/ansible-collection-zarf/issues/113)) ([df27962](https://github.com/colonel-byte/ansible-collection-zarf/commit/df279623798afcfaafbc00e0c2eb0bf4de8649c5))
* **deps:** Bump github.com/AzureAD/microsoft-authentication-library-for-go from 1.7.2 to 1.10.1 ([#96](https://github.com/colonel-byte/ansible-collection-zarf/issues/96)) ([cbe3190](https://github.com/colonel-byte/ansible-collection-zarf/commit/cbe3190b082c750ef21533716deb129a967e4c00))
* **deps:** Bump github.com/bmatcuk/doublestar/v4 from 4.10.0 to 4.10.3 ([#107](https://github.com/colonel-byte/ansible-collection-zarf/issues/107)) ([1784cfd](https://github.com/colonel-byte/ansible-collection-zarf/commit/1784cfdf911315f13e5fbbfbe9b221da8343cf77))
* **deps:** Bump github.com/bodgit/sevenzip from 1.6.1 to 1.6.5 ([#72](https://github.com/colonel-byte/ansible-collection-zarf/issues/72)) ([2d422da](https://github.com/colonel-byte/ansible-collection-zarf/commit/2d422da13118b70cba0027ec7254a9cb9eefccbc))
* **deps:** Bump github.com/buger/jsonparser from 1.1.2 to 1.6.1 ([#59](https://github.com/colonel-byte/ansible-collection-zarf/issues/59)) ([153f982](https://github.com/colonel-byte/ansible-collection-zarf/commit/153f9820bc824fc21ce310f90aee10a0c10f14f2))
* **deps:** Bump github.com/coreos/go-oidc/v3 from 3.18.0 to 3.21.0 ([#85](https://github.com/colonel-byte/ansible-collection-zarf/issues/85)) ([a7faac9](https://github.com/colonel-byte/ansible-collection-zarf/commit/a7faac9f5c514312254a6d4b9b78c05103be7913))
* **deps:** Bump github.com/envoyproxy/go-control-plane/envoy from 1.37.0 to 1.40.0 ([#134](https://github.com/colonel-byte/ansible-collection-zarf/issues/134)) ([3d2c9a0](https://github.com/colonel-byte/ansible-collection-zarf/commit/3d2c9a0e3a46c167aea5e3c310073b2358f7a301))
* **deps:** Bump github.com/fairwindsops/pluto/v5 from 5.24.3 to 5.24.4 ([#140](https://github.com/colonel-byte/ansible-collection-zarf/issues/140)) ([6c12965](https://github.com/colonel-byte/ansible-collection-zarf/commit/6c12965d720dfae32c880e22c28d4307ce9987cc))
* **deps:** Bump github.com/fluxcd/pkg/apis/meta from 1.31.0 to 1.36.0 ([#135](https://github.com/colonel-byte/ansible-collection-zarf/issues/135)) ([8c2ee36](https://github.com/colonel-byte/ansible-collection-zarf/commit/8c2ee36baf558aa3eb057d64eaccf0d71332ae12))
* **deps:** Bump github.com/fluxcd/source-controller/api from 1.9.5 to 1.9.6 ([#128](https://github.com/colonel-byte/ansible-collection-zarf/issues/128)) ([a3947db](https://github.com/colonel-byte/ansible-collection-zarf/commit/a3947dbc3bc6228c7c4d1b1702b081e56b37fe49))
* **deps:** Bump github.com/github/go-spdx/v2 from 2.7.0 to 2.7.1 ([#141](https://github.com/colonel-byte/ansible-collection-zarf/issues/141)) ([353e1ca](https://github.com/colonel-byte/ansible-collection-zarf/commit/353e1ca08cfc20de3c966e4835456fe361771f48))
* **deps:** Bump github.com/glebarez/go-sqlite from 1.22.0 to 1.23.0 ([#119](https://github.com/colonel-byte/ansible-collection-zarf/issues/119)) ([79f2452](https://github.com/colonel-byte/ansible-collection-zarf/commit/79f2452555f135473f7197afcdf0e097afa55103))
* **deps:** Bump github.com/gohugoio/hashstructure from 0.6.0 to 1.1.0 ([#111](https://github.com/colonel-byte/ansible-collection-zarf/issues/111)) ([a77f004](https://github.com/colonel-byte/ansible-collection-zarf/commit/a77f00437b17960325b38e3116a3167aeb9394a8))
* **deps:** Bump github.com/google/s2a-go from 0.1.10 to 0.1.11 ([#137](https://github.com/colonel-byte/ansible-collection-zarf/issues/137)) ([b49fac6](https://github.com/colonel-byte/ansible-collection-zarf/commit/b49fac68d5f1e50e254485df75b0e89a109ecdb5))
* **deps:** Bump github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp from 1.34.0 to 1.38.0 ([#109](https://github.com/colonel-byte/ansible-collection-zarf/issues/109)) ([67de312](https://github.com/colonel-byte/ansible-collection-zarf/commit/67de3127af980918034d6a2f3035d982ff9b8b49))
* **deps:** Bump github.com/hashicorp/aws-sdk-go-base/v2 from 2.0.0-beta.74 to 2.0.0-beta.76 ([#90](https://github.com/colonel-byte/ansible-collection-zarf/issues/90)) ([ec5d25e](https://github.com/colonel-byte/ansible-collection-zarf/commit/ec5d25e002a91554a36ab60bc653431828e08e24))
* **deps:** Bump github.com/hashicorp/go-getter from 1.8.8 to 1.8.10 ([#108](https://github.com/colonel-byte/ansible-collection-zarf/issues/108)) ([e10c7d2](https://github.com/colonel-byte/ansible-collection-zarf/commit/e10c7d2bc242845023c17f4939c48ee77e70ec34))
* **deps:** Bump github.com/huandu/xstrings from 1.5.0 to 1.6.2 ([#76](https://github.com/colonel-byte/ansible-collection-zarf/issues/76)) ([dd432c0](https://github.com/colonel-byte/ansible-collection-zarf/commit/dd432c0d3add6ef7e573ce3d21c8fa0400a068af))
* **deps:** Bump github.com/itchyny/timefmt-go from 0.1.7 to 0.1.9 ([#142](https://github.com/colonel-byte/ansible-collection-zarf/issues/142)) ([673ea0f](https://github.com/colonel-byte/ansible-collection-zarf/commit/673ea0f093e910a967c5d3a4ac8e9c5a155f0b5b))
* **deps:** Bump github.com/lestrrat-go/httprc/v3 from 3.0.5 to 3.0.6 ([#129](https://github.com/colonel-byte/ansible-collection-zarf/issues/129)) ([453d25b](https://github.com/colonel-byte/ansible-collection-zarf/commit/453d25b1544515838e12ec0e393b9aa224de1991))
* **deps:** Bump github.com/letsencrypt/boulder from 0.20260309.0 to 0.20261013.0 ([#156](https://github.com/colonel-byte/ansible-collection-zarf/issues/156)) ([9f45532](https://github.com/colonel-byte/ansible-collection-zarf/commit/9f45532bd0de0d4adcafea5136c7d4212aaee004))
* **deps:** Bump github.com/letsencrypt/boulder from 0.20261013.0 to 0.20261020.1 ([#161](https://github.com/colonel-byte/ansible-collection-zarf/issues/161)) ([3587b9d](https://github.com/colonel-byte/ansible-collection-zarf/commit/3587b9d5a801a4b50e68adff483ad5395a54ddd9))
* **deps:** Bump github.com/Microsoft/go-winio from 0.6.3 to 0.6.4 ([#42](https://github.com/colonel-byte/ansible-collection-zarf/issues/42)) ([326a387](https://github.com/colonel-byte/ansible-collection-zarf/commit/326a3873f2470ecd7943fa3e02061832fdceb8de))
* **deps:** Bump github.com/mikefarah/yq/v4 from 4.53.6 to 4.54.1 ([#94](https://github.com/colonel-byte/ansible-collection-zarf/issues/94)) ([37e3948](https://github.com/colonel-byte/ansible-collection-zarf/commit/37e394895d91c82d5436e545dfe84019b1b425ca))
* **deps:** Bump github.com/mozillazg/docker-credential-acr-helper from 0.4.0 to 0.4.2 ([#86](https://github.com/colonel-byte/ansible-collection-zarf/issues/86)) ([002d97c](https://github.com/colonel-byte/ansible-collection-zarf/commit/002d97cf17b8c7aa06cfac5813118d9a269b5dcf))
* **deps:** Bump github.com/open-policy-agent/opa from 1.17.1 to 1.21.2 ([#97](https://github.com/colonel-byte/ansible-collection-zarf/issues/97)) ([797e82f](https://github.com/colonel-byte/ansible-collection-zarf/commit/797e82f8b4efb7581b3f52daa32fb9701dd33df7))
* **deps:** Bump github.com/pandatix/go-cvss from 0.6.2 to 0.6.4 ([#74](https://github.com/colonel-byte/ansible-collection-zarf/issues/74)) ([48722d4](https://github.com/colonel-byte/ansible-collection-zarf/commit/48722d4f02a390733e77fffa2897328d8e764b7d))
* **deps:** Bump github.com/pb33f/go-yaml from 0.1.0 to 0.1.1 ([#69](https://github.com/colonel-byte/ansible-collection-zarf/issues/69)) ([7bd2fa1](https://github.com/colonel-byte/ansible-collection-zarf/commit/7bd2fa1091ce9b4012e67c4b7ae503ec78a1a03d))
* **deps:** Bump github.com/pb33f/ordered-map/v2 from 2.3.1 to 2.3.2 ([#57](https://github.com/colonel-byte/ansible-collection-zarf/issues/57)) ([c0d96f6](https://github.com/colonel-byte/ansible-collection-zarf/commit/c0d96f69d0301a98867f9d97c2792be5731d1f52))
* **deps:** Bump github.com/sahilm/fuzzy from 0.1.1 to 0.1.3 ([#138](https://github.com/colonel-byte/ansible-collection-zarf/issues/138)) ([5f269a9](https://github.com/colonel-byte/ansible-collection-zarf/commit/5f269a9ca8e3d9b5adfb4cb691e9a5121cc11d1a))
* **deps:** Bump github.com/shopspring/decimal from 1.4.0 to 1.6.0 ([#133](https://github.com/colonel-byte/ansible-collection-zarf/issues/133)) ([9346f34](https://github.com/colonel-byte/ansible-collection-zarf/commit/9346f34a68e54ddb59424d543f50bac18a45946a))
* **deps:** Bump github.com/smallnest/ringbuffer from 0.0.0-20241116012123-461381446e3d to 0.2.0 ([#81](https://github.com/colonel-byte/ansible-collection-zarf/issues/81)) ([385a51c](https://github.com/colonel-byte/ansible-collection-zarf/commit/385a51c6459349b3e0be0c6cc5a0ead81af1e157))
* **deps:** Bump github.com/spiffe/go-spiffe/v2 from 2.8.1 to 2.9.0 ([#132](https://github.com/colonel-byte/ansible-collection-zarf/issues/132)) ([85716a8](https://github.com/colonel-byte/ansible-collection-zarf/commit/85716a88d56b248c76d4584c19a29d85f86c981f))
* **deps:** Bump github.com/ulikunitz/xz from 0.5.16 to 0.5.17 ([#100](https://github.com/colonel-byte/ansible-collection-zarf/issues/100)) ([5655836](https://github.com/colonel-byte/ansible-collection-zarf/commit/56558361b31c7ede066e27f9094968d48d620c7a))
* **deps:** Bump google.golang.org/api from 0.299.0 to 0.304.0 ([#136](https://github.com/colonel-byte/ansible-collection-zarf/issues/136)) ([5928499](https://github.com/colonel-byte/ansible-collection-zarf/commit/59284990883815aea24d2db87511e45b0e47ac63))
* **deps:** Bump gorm.io/gorm from 1.31.1 to 1.31.2 ([#116](https://github.com/colonel-byte/ansible-collection-zarf/issues/116)) ([1e8a96a](https://github.com/colonel-byte/ansible-collection-zarf/commit/1e8a96afe81f0d1e4a93bd8a70a81e04a79a1737))
* **deps:** Bump helm.sh/helm/v3 from 3.21.3 to 3.22.1 ([#112](https://github.com/colonel-byte/ansible-collection-zarf/issues/112)) ([5abdaf0](https://github.com/colonel-byte/ansible-collection-zarf/commit/5abdaf09ac200da74f95f55c5500197083562ce6))
* **deps:** Bump https://github.com/astral-sh/ruff-pre-commit from v0.17.0 to 0.17.1 ([#41](https://github.com/colonel-byte/ansible-collection-zarf/issues/41)) ([19b865a](https://github.com/colonel-byte/ansible-collection-zarf/commit/19b865a86ea4916deaaa3f6a01df81a58066831d))
* **deps:** Bump https://github.com/astral-sh/ruff-pre-commit from v0.17.1 to 0.17.2 ([#159](https://github.com/colonel-byte/ansible-collection-zarf/issues/159)) ([562910d](https://github.com/colonel-byte/ansible-collection-zarf/commit/562910db1266aaa4458b70a7222c3d7eb2b97200))
* **deps:** Bump https://github.com/golangci/golangci-lint from v2.14.0 to 2.15.0 ([#58](https://github.com/colonel-byte/ansible-collection-zarf/issues/58)) ([56aac91](https://github.com/colonel-byte/ansible-collection-zarf/commit/56aac91d02f819832862a15216f1b08b5e90cd13))
* **deps:** Bump https://github.com/python-jsonschema/check-jsonschema from 0.35.0 to 0.38.2 ([#60](https://github.com/colonel-byte/ansible-collection-zarf/issues/60)) ([8a783d4](https://github.com/colonel-byte/ansible-collection-zarf/commit/8a783d467d2b0b55aa6ee09a4cbd2dabdaf22236))
* **deps:** Bump https://github.com/sirosen/texthooks from 0.7.1 to 1.0.0 ([#40](https://github.com/colonel-byte/ansible-collection-zarf/issues/40)) ([55e0a51](https://github.com/colonel-byte/ansible-collection-zarf/commit/55e0a51ba39edc490d6914e0a2f4d7c11216903d))
* **deps:** Bump j178/prek-action from 3.0.1 to 3.0.2 in the core group across 1 directory ([#77](https://github.com/colonel-byte/ansible-collection-zarf/issues/77)) ([4a57f08](https://github.com/colonel-byte/ansible-collection-zarf/commit/4a57f08b836ac16e80bca01b4876f0e59eb7622b))
* **deps:** Bump j178/prek-action from 3.0.2 to 3.1.0 in the core group ([#157](https://github.com/colonel-byte/ansible-collection-zarf/issues/157)) ([249efe8](https://github.com/colonel-byte/ansible-collection-zarf/commit/249efe8dca08def130a67b4c0d1d075f58fcb39d))
* **deps:** Bump modernc.org/libc from 1.74.4 to 1.79.0 ([#93](https://github.com/colonel-byte/ansible-collection-zarf/issues/93)) ([e28ac25](https://github.com/colonel-byte/ansible-collection-zarf/commit/e28ac253e89892220dfc3626408faa67af797ffc))
* **deps:** Bump the core group with 3 updates ([#43](https://github.com/colonel-byte/ansible-collection-zarf/issues/43)) ([8e7d745](https://github.com/colonel-byte/ansible-collection-zarf/commit/8e7d745cff172b8d740d7289a61d782df6fa342d))
* **deps:** Bump the core group with 5 updates ([#160](https://github.com/colonel-byte/ansible-collection-zarf/issues/160)) ([e8b9bdb](https://github.com/colonel-byte/ansible-collection-zarf/commit/e8b9bdb1fc7034e26e2b40a0e1ff62c1dc5d3209))
* **deps:** Bump the openapi group with 8 updates ([#127](https://github.com/colonel-byte/ansible-collection-zarf/issues/127)) ([0b7f50e](https://github.com/colonel-byte/ansible-collection-zarf/commit/0b7f50e4e4a37d2dcc6ca4a638812898f1e4d5f9))
* **deps:** Bump zarf-dev/zarf/agent from v0.87.0 to v0.88.0 in /containers/ansible in the containers group ([#36](https://github.com/colonel-byte/ansible-collection-zarf/issues/36)) ([f00e08f](https://github.com/colonel-byte/ansible-collection-zarf/commit/f00e08ff15eb52e91fcffd9f99afb935d35d289b))
* **deps:** group every Go module update into one Dependabot branch ([#155](https://github.com/colonel-byte/ansible-collection-zarf/issues/155)) ([06c8ac2](https://github.com/colonel-byte/ansible-collection-zarf/commit/06c8ac29d5f027881d5ac72e20b77411d6b9d12a))

## [0.87.0](https://github.com/colonel-byte/ansible-collection-zarf/compare/v0.86.0...v0.87.0) (2026-10-15)


### ⚠ BREAKING CHANGES

* **cosign:** update public key ([#22](https://github.com/colonel-byte/ansible-collection-zarf/issues/22))

### Features

* **collection:** add the colonel_byte.zarf Ansible collection ([#11](https://github.com/colonel-byte/ansible-collection-zarf/issues/11)) ([6f0e2d1](https://github.com/colonel-byte/ansible-collection-zarf/commit/6f0e2d14ba26288bff00c501dcda2c0bb8b8e72f))
* **containers:** build the management node image here ([#27](https://github.com/colonel-byte/ansible-collection-zarf/issues/27)) ([97fa987](https://github.com/colonel-byte/ansible-collection-zarf/commit/97fa9879e1c14885e0d0594e03e2656175a03a9e))
* init ([412fcbf](https://github.com/colonel-byte/ansible-collection-zarf/commit/412fcbf01e7d2033f0e30227c6b72c2541694d50))
* **zarfmod:** add the module logic and the two entry points ([#12](https://github.com/colonel-byte/ansible-collection-zarf/issues/12)) ([e745949](https://github.com/colonel-byte/ansible-collection-zarf/commit/e745949f59a173c019f73afcad3820007ae05ba1))


### Documentation

* correct the mage target names the guide documents ([#21](https://github.com/colonel-byte/ansible-collection-zarf/issues/21)) ([7a4dbc4](https://github.com/colonel-byte/ansible-collection-zarf/commit/7a4dbc400e57dfad8e251cb30c37dab02b90623b))


### CI/CD

* **codeql:** stop analysing the merge queue ref ([#26](https://github.com/colonel-byte/ansible-collection-zarf/issues/26)) ([c6315fd](https://github.com/colonel-byte/ansible-collection-zarf/commit/c6315fd531a1215c1e01a9e88f32d7394059c74b))
* **cosign:** update public key ([#22](https://github.com/colonel-byte/ansible-collection-zarf/issues/22)) ([cbe539c](https://github.com/colonel-byte/ansible-collection-zarf/commit/cbe539c456cc347c64b64364abe84e39d8890ece))
* **release:** let release-please move the image tag ([#28](https://github.com/colonel-byte/ansible-collection-zarf/issues/28)) ([94d57da](https://github.com/colonel-byte/ansible-collection-zarf/commit/94d57da060f54405b4c1f11e7ddff2f916ade0b0))
* update zarf action setup ([#19](https://github.com/colonel-byte/ansible-collection-zarf/issues/19)) ([14691ca](https://github.com/colonel-byte/ansible-collection-zarf/commit/14691ca09e9f6ddaa6574603900e8552d5494211))


### Build

* **deps:** Bump the core group with 2 updates ([#18](https://github.com/colonel-byte/ansible-collection-zarf/issues/18)) ([bf647ae](https://github.com/colonel-byte/ansible-collection-zarf/commit/bf647ae37b0df5b85c423e4015e72f865b61d5b8))
* **deps:** Bump the core group with 2 updates ([#23](https://github.com/colonel-byte/ansible-collection-zarf/issues/23)) ([739f112](https://github.com/colonel-byte/ansible-collection-zarf/commit/739f112deeb07af1b25b10fc866802eb01282a34))
* **mage:** add the task runner and the book's generators ([#15](https://github.com/colonel-byte/ansible-collection-zarf/issues/15)) ([f54f5e7](https://github.com/colonel-byte/ansible-collection-zarf/commit/f54f5e74b88e4556ecd9dbb00dffcfdafbc51ea1))
* **release:** track the zarf version line, starting at 0.87.0 ([#20](https://github.com/colonel-byte/ansible-collection-zarf/issues/20)) ([03feec6](https://github.com/colonel-byte/ansible-collection-zarf/commit/03feec6a6d2e5f566df6af2e87776ea1de34689d))
* vendor the dependency tree and pin the toolchain to it ([#16](https://github.com/colonel-byte/ansible-collection-zarf/issues/16)) ([a025841](https://github.com/colonel-byte/ansible-collection-zarf/commit/a025841f94426e26c36a763c7eef222f35230d7e))
