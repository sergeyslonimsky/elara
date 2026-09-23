# Changelog

All notable changes to Elara are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Entries are generated from [Conventional Commits](https://www.conventionalcommits.org/)
by [git-cliff](https://git-cliff.org) — see `cliff.toml`.

## [0.5.0](https://github.com/sergeyslonimsky/elara/releases/tag/v0.5.0) — 2026-09-23

### ⚠ Breaking changes

- **auth:** Elara now refuses to start on two configurations it previously accepted: DANGEROUSLY_SKIP_PERMISSIONS=true together with UI_AUTH_ENABLED=true or CLIENT_AUTH_ENABLED=true, and any UI_AUTH_TYPE value outside oidc / basic-auth / none / empty. Both previously ran with authorization silently disabled.
- **etcd:** Writes through the etcd-compatible gRPC API are now validated against the namespace's JSON Schema. Clients that were writing values which do not satisfy an attached schema will start receiving errors on Put.

### Features

- **cli:** Add `elara version` and stamp build metadata into releases ([3213b58](https://github.com/sergeyslonimsky/elara/commit/3213b5866e43d2ba30edd1742cf470be8c6d277a))
- **etcd:** Route the KV server through the config usecase ([305ac43](https://github.com/sergeyslonimsky/elara/commit/305ac4306a4f5ab610bf84942ca79ddbfb3367c1))

### Bug fixes

- **auth:** Derive permission-skip from auth type, reject misleading configs ([bcbda51](https://github.com/sergeyslonimsky/elara/commit/bcbda51ec25b44b04a55a3a43e3d4cb09183fcfd))
- **web:** Correct user-action and route visibility across auth modes ([6880895](https://github.com/sergeyslonimsky/elara/commit/688089595370b831e333f49c04ee3cc57f7e835e))
- **helm:** Drop session values the service never reads ([cc44a4e](https://github.com/sergeyslonimsky/elara/commit/cc44a4ec4bee97abd32cd62e82a0723eb61c4267))
- **ci:** Run the integration suites and attribute their coverage ([a72a110](https://github.com/sergeyslonimsky/elara/commit/a72a11052b9342032920393d6cff4187db1ec672))

### Refactoring

- **handler:** Extract shared ProtoSortToDomain helper ([5755c1c](https://github.com/sergeyslonimsky/elara/commit/5755c1cc2ed0d3c312421c966b11be37814f93f0))
- Drop unused domain methods and integration-test helpers ([03876da](https://github.com/sergeyslonimsky/elara/commit/03876da8b51e093f557bdb9372a4003755894679))

### Documentation

- Restructure the site into sectioned navigation ([844acce](https://github.com/sergeyslonimsky/elara/commit/844acced98652d27b70ad6c73161b60443f5decf))
- Correct claims the etcd and auth changes invalidated ([8dfc810](https://github.com/sergeyslonimsky/elara/commit/8dfc810cf4a609c3fcb6e18dc36380902100ab78))
- Document the CLI, close the remaining audit gaps ([bc69371](https://github.com/sergeyslonimsky/elara/commit/bc69371e70e959624e90859df6dec2db9ac3e2da))

**Full diff:** [v0.4.0...v0.5.0](https://github.com/sergeyslonimsky/elara/compare/v0.4.0...v0.5.0)

## [0.4.0](https://github.com/sergeyslonimsky/elara/releases/tag/v0.4.0) — 2026-08-27

### Features

- Support local ~/.elara install (go install / downloadable binaries) ([#106](https://github.com/sergeyslonimsky/elara/pull/106)) ([3551ba3](https://github.com/sergeyslonimsky/elara/commit/3551ba3319f05ac8e9f4aba3f15c924998b1a2c2))

**Full diff:** [v0.3.3...v0.4.0](https://github.com/sergeyslonimsky/elara/compare/v0.3.3...v0.4.0)

## [0.3.3](https://github.com/sergeyslonimsky/elara/releases/tag/v0.3.3) — 2026-08-02

### Bug fixes

- **casbin:** Sync in-memory cache with bbolt on user/group delete ([#79](https://github.com/sergeyslonimsky/elara/pull/79)) ([e89ea75](https://github.com/sergeyslonimsky/elara/commit/e89ea758de667c26a954891105751cdbfa0810d7))
- **ci:** Quote the mkdocs pip install command in docs.yml ([5d10c11](https://github.com/sergeyslonimsky/elara/commit/5d10c11c8d245520eccdccfdff8da817f94d8954))

### Documentation

- **readme:** Add Deploy to Kubernetes section, link Helm chart docs ([20ef65f](https://github.com/sergeyslonimsky/elara/commit/20ef65f171cfe80ba86b73b65486c92de3929628))
- Add Concepts, Authentication, Configuration pages; restructure nav ([#84](https://github.com/sergeyslonimsky/elara/pull/84)) ([b16653e](https://github.com/sergeyslonimsky/elara/commit/b16653e996e3fefbb18b20f30c5ce1f02315b106))

### Testing

- Close SonarCloud coverage gate gap on master ([#78](https://github.com/sergeyslonimsky/elara/pull/78)) ([c65e0c7](https://github.com/sergeyslonimsky/elara/commit/c65e0c72271c62f630f5212d6b58d447cffdba53))

### Other changes

- Watch RBAC fix + capabilities endpoint for gated UI features ([#75](https://github.com/sergeyslonimsky/elara/pull/75)) ([645acde](https://github.com/sergeyslonimsky/elara/commit/645acde1324473e2909fe808c21083f2f530b8e1))
- OSS launch prep — README, ADRs, docs site, demo mode, todo-app example ([#83](https://github.com/sergeyslonimsky/elara/pull/83)) ([7bd8033](https://github.com/sergeyslonimsky/elara/commit/7bd80337d99fd219d4949acbdbb5e32fbc9a384f))

**Full diff:** [v0.3.2...v0.3.3](https://github.com/sergeyslonimsky/elara/compare/v0.3.2...v0.3.3)

## [0.3.2](https://github.com/sergeyslonimsky/elara/releases/tag/v0.3.2) — 2026-06-11

### Bug fixes

- **auth:** Fix oidc auth flow ([04a243e](https://github.com/sergeyslonimsky/elara/commit/04a243e135010db85f2b6202523e6587c83bcfc2))

**Full diff:** [v0.3.1...v0.3.2](https://github.com/sergeyslonimsky/elara/compare/v0.3.1...v0.3.2)

## [0.3.1](https://github.com/sergeyslonimsky/elara/releases/tag/v0.3.1) — 2026-06-10

### Bug fixes

- Fix no-auth error ([07d1ba4](https://github.com/sergeyslonimsky/elara/commit/07d1ba4d8c292c35b149109f9e1dbb524d32777c))
- Fix(auth): make OIDC login work on a fresh bbolt store ([a31adff](https://github.com/sergeyslonimsky/elara/commit/a31adff4ad451cb43b81c2e7dccac04bcee94403))
- **sonar:** Fix some sonar issues, add tests ([c29a680](https://github.com/sergeyslonimsky/elara/commit/c29a68031632ad32893de95b63810e4a1128baf4))
- **codecov:** Ignore codecov on master ([392569e](https://github.com/sergeyslonimsky/elara/commit/392569ed23b26bd0d2b83bacfb432efe6c26d654))

**Full diff:** [v0.3.0...v0.3.1](https://github.com/sergeyslonimsky/elara/compare/v0.3.0...v0.3.1)

## [0.3.0](https://github.com/sergeyslonimsky/elara/releases/tag/v0.3.0) — 2026-06-10

### Features

- Add webhook push notifications on config changes ([#38](https://github.com/sergeyslonimsky/elara/pull/38)) ([d305451](https://github.com/sergeyslonimsky/elara/commit/d305451d1796fdeb7502c60dc3f3299732b6887e))
- Add user auth flow ([#42](https://github.com/sergeyslonimsky/elara/pull/42)) ([a8c7007](https://github.com/sergeyslonimsky/elara/commit/a8c7007b301ae5842b516e9a2f6659d62c104e3b))
- **auth:** Implement transitive group permissions ([#54](https://github.com/sergeyslonimsky/elara/pull/54)) ([a2ce422](https://github.com/sergeyslonimsky/elara/commit/a2ce422592afe2b6811488eccb254f9ea270d72a))
- **auth:** Add delete user RPC and support OIDC user creation ([#55](https://github.com/sergeyslonimsky/elara/pull/55)) ([881518f](https://github.com/sergeyslonimsky/elara/commit/881518f73215a42028df20ad6f8b59ef2e152dac))

### Bug fixes

- Add gpg key for helm signing ([#32](https://github.com/sergeyslonimsky/elara/pull/32)) ([49f7a13](https://github.com/sergeyslonimsky/elara/commit/49f7a13a300b0d05ea0b172c71ccbc18ec4464a7))

### Refactoring

- Add contrib docs ([7fa6337](https://github.com/sergeyslonimsky/elara/commit/7fa6337516b56f03ad7b7abe211e21269a13c79a))
- Add tests for frontend ([#39](https://github.com/sergeyslonimsky/elara/pull/39)) ([76167e9](https://github.com/sergeyslonimsky/elara/commit/76167e9cdf3c2fbb2cddf89dbd6eeb5e7f9c12bb))
- Add fe tests v1 ([#40](https://github.com/sergeyslonimsky/elara/pull/40)) ([9325bb0](https://github.com/sergeyslonimsky/elara/commit/9325bb0304c1b3018dc3e6aaf622a2d22d92acdf))
- Add codecov test analysis ([#41](https://github.com/sergeyslonimsky/elara/pull/41)) ([d1b4b0f](https://github.com/sergeyslonimsky/elara/commit/d1b4b0fabea05fee64279185fdbf9cbc0e30b033))

### Documentation

- Update README, add exmaples how to connect using etcd client ([25e15f0](https://github.com/sergeyslonimsky/elara/commit/25e15f046114823e0bb6e842afb261596428b457))
- Add webhooks section to README ([b907c06](https://github.com/sergeyslonimsky/elara/commit/b907c06e94907596506dcb05accee9869cb3e096))
- Add codecov badge ([18694bb](https://github.com/sergeyslonimsky/elara/commit/18694bba128a7f47d9885ecb7c977f90f926cd0b))

### Build & CI

- **deps-dev:** Bump the dev group in /web with 2 updates ([#34](https://github.com/sergeyslonimsky/elara/pull/34)) ([8d3044d](https://github.com/sergeyslonimsky/elara/commit/8d3044d63c2139ac28a7eb8d41642694ba198f98))

### Other changes

- Add Contributor Covenant Code of Conduct ([c8cded1](https://github.com/sergeyslonimsky/elara/commit/c8cded106dc3f675e3a53578f5d18aa1dc7cbcb7))
- Feat user management fe ([#67](https://github.com/sergeyslonimsky/elara/pull/67)) ([07b93ef](https://github.com/sergeyslonimsky/elara/commit/07b93ef34b51e8d8af0d4014f70efd70ca1b56e3))

**Full diff:** [v0.2.0-rc.4...v0.3.0](https://github.com/sergeyslonimsky/elara/compare/v0.2.0-rc.4...v0.3.0)

## [0.2.0-rc.4](https://github.com/sergeyslonimsky/elara/releases/tag/v0.2.0-rc.4) — 2026-04-26

### Bug fixes

- Add helm signing ([773c4ba](https://github.com/sergeyslonimsky/elara/commit/773c4baad00fb43036b2b2893a53e0d25cd85289))
- Add helm signing ([83d9a64](https://github.com/sergeyslonimsky/elara/commit/83d9a646b3162b7cbf631b7cb68cb3b2db67ffd3))

**Full diff:** [v0.2.0-rc.3...v0.2.0-rc.4](https://github.com/sergeyslonimsky/elara/compare/v0.2.0-rc.3...v0.2.0-rc.4)

## [0.2.0-rc.3](https://github.com/sergeyslonimsky/elara/releases/tag/v0.2.0-rc.3) — 2026-04-26

### Bug fixes

- Fix ci docker verion (change to sha) ([7c5e1b3](https://github.com/sergeyslonimsky/elara/commit/7c5e1b3f03498b0d0570023a73caee44747fae29))
- Add helm signing ([#31](https://github.com/sergeyslonimsky/elara/pull/31)) ([4f625bc](https://github.com/sergeyslonimsky/elara/commit/4f625bcf6a52e0c530fbc76c5abcd56b2b713bb7))

**Full diff:** [v0.2.0-rc.2...v0.2.0-rc.3](https://github.com/sergeyslonimsky/elara/compare/v0.2.0-rc.2...v0.2.0-rc.3)

## [0.2.0-rc.2](https://github.com/sergeyslonimsky/elara/releases/tag/v0.2.0-rc.2) — 2026-04-26

### Bug fixes

- Add docker login to release ci ([f645f3d](https://github.com/sergeyslonimsky/elara/commit/f645f3d09e8e34b0446271e93008c04c399bc433))

**Full diff:** [v0.2.0-rc.1...v0.2.0-rc.2](https://github.com/sergeyslonimsky/elara/compare/v0.2.0-rc.1...v0.2.0-rc.2)

## [0.2.0-rc.1](https://github.com/sergeyslonimsky/elara/releases/tag/v0.2.0-rc.1) — 2026-04-26

### Features

- Add JSON Schema validation for configs ([#29](https://github.com/sergeyslonimsky/elara/pull/29)) ([9a3368a](https://github.com/sergeyslonimsky/elara/commit/9a3368a941b1cd9339de552604eae79682b1310c))

### Refactoring

- Refactor frontend structure ([#30](https://github.com/sergeyslonimsky/elara/pull/30)) ([7478ac1](https://github.com/sergeyslonimsky/elara/commit/7478ac114eb1e21ec522718c5809ace7756b2651))

### Build & CI

- Add new ci job for signing helm release ([#22](https://github.com/sergeyslonimsky/elara/pull/22)) ([8e47ce5](https://github.com/sergeyslonimsky/elara/commit/8e47ce59b6ba321ee502cd5598ea1236ced4841e))

**Full diff:** [v0.2.0-rc.0...v0.2.0-rc.1](https://github.com/sergeyslonimsky/elara/compare/v0.2.0-rc.0...v0.2.0-rc.1)

## [0.2.0-rc.0](https://github.com/sergeyslonimsky/elara/releases/tag/v0.2.0-rc.0) — 2026-04-22

### Features

- Add config revision diff ([#13](https://github.com/sergeyslonimsky/elara/pull/13)) ([9e8698c](https://github.com/sergeyslonimsky/elara/commit/9e8698c1f2a218612f18e2ea3bb231a69e879252))
- Configs export/import ([#18](https://github.com/sergeyslonimsky/elara/pull/18)) ([0286cf2](https://github.com/sergeyslonimsky/elara/commit/0286cf2cc09ca1e0ab3fa46bde36f1f995a21441))
- Add coonfig/namespace lock/unlock feature. ([#20](https://github.com/sergeyslonimsky/elara/pull/20)) ([5955a1d](https://github.com/sergeyslonimsky/elara/commit/5955a1d43ae7f279803940cbbb0223a1eaaf457b))

### Bug fixes

- Fix logo size ([4b09088](https://github.com/sergeyslonimsky/elara/commit/4b09088922de863c6deea75701f462f5be8616d9))
- Fix: ([f73588a](https://github.com/sergeyslonimsky/elara/commit/f73588a91c0f17873e8ed63a13aad4d4b3e98636))
- Refactor CI process ([#16](https://github.com/sergeyslonimsky/elara/pull/16)) ([44686a8](https://github.com/sergeyslonimsky/elara/commit/44686a8c5d9c08a6615347fdedc1afce3c19930c))

### Refactoring

- Update readme file ([#17](https://github.com/sergeyslonimsky/elara/pull/17)) ([9a0081e](https://github.com/sergeyslonimsky/elara/commit/9a0081e642d73100378fafacce185435c2efcc2b))
- Update CI ([0ec7cc5](https://github.com/sergeyslonimsky/elara/commit/0ec7cc5732ea4fa45fcedd78f9e54da15e0b8953))
- Add config for logger (level, format, noSource) ([#19](https://github.com/sergeyslonimsky/elara/pull/19)) ([93a777d](https://github.com/sergeyslonimsky/elara/commit/93a777d3cc89811eef55b56117c71e76be71ec27))

### Documentation

- Update readme with code quality badges ([55328f3](https://github.com/sergeyslonimsky/elara/commit/55328f3eae31e71c56ba6101c3130fb67258319c))

### Build & CI

- **deps-dev:** Bump vite from 8.0.8 to 8.0.9 in /web in the dev group ([#14](https://github.com/sergeyslonimsky/elara/pull/14)) ([615a79d](https://github.com/sergeyslonimsky/elara/commit/615a79d1660fd5ba583d7fc7e93364fcfbb53180))

### Other changes

- Optimize FE part ([#21](https://github.com/sergeyslonimsky/elara/pull/21)) ([edf8e3a](https://github.com/sergeyslonimsky/elara/commit/edf8e3adf7d420260f3bee9d98d2852feb3da7d2))

**Full diff:** [v0.1.0-rc.0...v0.2.0-rc.0](https://github.com/sergeyslonimsky/elara/compare/v0.1.0-rc.0...v0.2.0-rc.0)

## [0.1.0-rc.0](https://github.com/sergeyslonimsky/elara/releases/tag/v0.1.0-rc.0) — 2026-04-18

### Bug fixes

- Setup node and build web code for fixing go vet linter check ([03287d1](https://github.com/sergeyslonimsky/elara/commit/03287d1b6a64b24bc9831f17d2cb6e5dc027d567))

### Other changes

- Init ([e51ad13](https://github.com/sergeyslonimsky/elara/commit/e51ad1301e2b8ce997c3bc28055c979c9c737581))


