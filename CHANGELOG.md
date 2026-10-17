# Changelog

## [0.87.1](https://github.com/colonel-byte/ansible-collection-zarf/compare/v0.87.0...v0.87.1) (2026-10-17)


### CI/CD

* **fuzz:** add fuzzing to golang ([#38](https://github.com/colonel-byte/ansible-collection-zarf/issues/38)) ([39cff2a](https://github.com/colonel-byte/ansible-collection-zarf/commit/39cff2a3f115c61f9ec27df132b91e2a053e3113))


### Build

* **deps:** Bump zarf-dev/zarf/agent from v0.87.0 to v0.88.0 in /containers/ansible in the containers group ([#36](https://github.com/colonel-byte/ansible-collection-zarf/issues/36)) ([f00e08f](https://github.com/colonel-byte/ansible-collection-zarf/commit/f00e08ff15eb52e91fcffd9f99afb935d35d289b))

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
