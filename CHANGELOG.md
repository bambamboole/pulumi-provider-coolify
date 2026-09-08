# Changelog

## [0.12.1](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.12.0...v0.12.1) (2026-09-08)


### Bug Fixes

* keep resource UUIDs known in update previews and accept unknown owners in Check ([#33](https://github.com/bambamboole/pulumi-provider-coolify/issues/33)) ([07326c3](https://github.com/bambamboole/pulumi-provider-coolify/commit/07326c39a9c2d47430be25cf6316b195ca50ed9f))

## [0.12.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.11.0...v0.12.0) (2026-09-07)


### Features

* compare private keys, GitHub Apps and compose files against Coolify instead of state ([#32](https://github.com/bambamboole/pulumi-provider-coolify/issues/32)) ([b2f8d33](https://github.com/bambamboole/pulumi-provider-coolify/commit/b2f8d336a9c61503f18f38f7ef1eb18512ba0380))
* resolve private key on read and make GitHub App clientSecret optional ([#30](https://github.com/bambamboole/pulumi-provider-coolify/issues/30)) ([ab12a9e](https://github.com/bambamboole/pulumi-provider-coolify/commit/ab12a9e801c8c016f701e9b82efa7f71e46526cb))

## [0.11.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.10.0...v0.11.0) (2026-09-07)


### Features

* **application:** compose location and domains, build settings, environment value overwrite ([#28](https://github.com/bambamboole/pulumi-provider-coolify/issues/28)) ([6af39cf](https://github.com/bambamboole/pulumi-provider-coolify/commit/6af39cf96500c202bff2927d98517d3c0f36a0ba))

## [0.10.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.9.0...v0.10.0) (2026-09-07)


### Features

* add service deployment triggers and preserve domain ports ([#27](https://github.com/bambamboole/pulumi-provider-coolify/issues/27)) ([34e4d7a](https://github.com/bambamboole/pulumi-provider-coolify/commit/34e4d7a07ef3fd18266c9fd58361db33cbbfa4db))
* manage service domains through native Coolify API ([#25](https://github.com/bambamboole/pulumi-provider-coolify/issues/25)) ([b7724c6](https://github.com/bambamboole/pulumi-provider-coolify/commit/b7724c6534ba20505620411a3b894796a65432a0))

## [0.9.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.8.1...v0.9.0) (2026-09-06)


### Features

* manage shared variables across all scopes ([4bcb46e](https://github.com/bambamboole/pulumi-provider-coolify/commit/4bcb46e853cb54f58a006d0c123ea2c11c62664b))

## [0.8.1](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.8.0...v0.8.1) (2026-09-06)


### Bug Fixes

* support root team notification settings ([c4d3282](https://github.com/bambamboole/pulumi-provider-coolify/commit/c4d328266e37315abca1a30d3288ed517866992d))

## [0.8.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.7.1...v0.8.0) (2026-09-06)


### Features

* manage team notification channels ([b56bcf3](https://github.com/bambamboole/pulumi-provider-coolify/commit/b56bcf3d3f4619db61d5398cb5e9c0727c44d4a4))

## [0.7.1](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.7.0...v0.7.1) (2026-09-06)


### Bug Fixes

* preserve resources when upgrading tag support ([d971849](https://github.com/bambamboole/pulumi-provider-coolify/commit/d971849d55a38f516d4ad9122e887fce58f13966))

## [0.7.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.6.0...v0.7.0) (2026-09-06)


### Features

* add Storage and VolumeBackup resources and a getStorage function ([e84fe43](https://github.com/bambamboole/pulumi-provider-coolify/commit/e84fe435b189f926522dc7e7e183a87b8d01cb2e))
* manage tags on applications, databases and services with provider default tags ([bec3af7](https://github.com/bambamboole/pulumi-provider-coolify/commit/bec3af74ba04d665fa70d45da6d081a2dcfd2fdc))

## [0.6.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.5.0...v0.6.0) (2026-09-06)


### Features

* add DatabaseBackup and Service resources and move resources between projects in place ([ecf90e2](https://github.com/bambamboole/pulumi-provider-coolify/commit/ecf90e2ce532c8407d0dea600422326dc0df7a6e))

## [0.5.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.4.2...v0.5.0) (2026-09-06)


### ⚠ BREAKING CHANGES

* Database and Application take projectUuid and environmentName instead of project and environment names; Database outputs username, password, databaseName, internalUrl and externalUrl replace the postgres-specific outputs; Application.source is required; Deployment Read no longer errors on pruned deployments. Existing databases and applications are replaced on the next update.
* the internal client API changed; resources are updated in the following commit.

### Features

* generate the Coolify API client from the OpenAPI specification ([d440a7e](https://github.com/bambamboole/pulumi-provider-coolify/commit/d440a7e06cae4e92ed1b10c16eb5b035c1a5eeeb))
* reference by UUID, reconcile every field and add GitHubApp and ScheduledTask ([9075a16](https://github.com/bambamboole/pulumi-provider-coolify/commit/9075a162e57d0590762527cdd5861731dfa6c5e1))


### Bug Fixes

* **release:** bump the SDK's pulumi.version alongside its package version ([430eef1](https://github.com/bambamboole/pulumi-provider-coolify/commit/430eef1a3083ee8af8e779a77c1dd26b3fdc356f))

## [0.4.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/v0.3.2...v0.4.0) (2026-09-05)


### Features

* native Pulumi provider for Coolify v4 ([3e21b47](https://github.com/bambamboole/pulumi-provider-coolify/commit/3e21b4752250b03b38c89c44723f30418b823d8a))
* support pull request previews and docker tag overrides on Deployment ([c812f4e](https://github.com/bambamboole/pulumi-provider-coolify/commit/c812f4ef2ce254c62c6966575e1e8461d49c4843))


### Bug Fixes

* build release artifacts on the release-please tag ([afe2845](https://github.com/bambamboole/pulumi-provider-coolify/commit/afe2845c4a2d4ad608f0770510324effd2b53914))
* **release:** install goreleaser without running a release ([e393fa0](https://github.com/bambamboole/pulumi-provider-coolify/commit/e393fa09a9287010d9b461092848123dc2481e0d))
* **release:** tag releases with plain version (vX.Y.Z) ([3738347](https://github.com/bambamboole/pulumi-provider-coolify/commit/37383471373a654e6ccb6f16271e43407ef0fa32))

## [0.3.2](https://github.com/bambamboole/pulumi-provider-coolify/compare/pulumi-provider-coolify-v0.3.1...pulumi-provider-coolify-v0.3.2) (2026-09-05)


### Bug Fixes

* **release:** install goreleaser without running a release ([e393fa0](https://github.com/bambamboole/pulumi-provider-coolify/commit/e393fa09a9287010d9b461092848123dc2481e0d))

## [0.3.1](https://github.com/bambamboole/pulumi-provider-coolify/compare/pulumi-provider-coolify-v0.3.0...pulumi-provider-coolify-v0.3.1) (2026-09-05)


### Bug Fixes

* build release artifacts on the release-please tag ([afe2845](https://github.com/bambamboole/pulumi-provider-coolify/commit/afe2845c4a2d4ad608f0770510324effd2b53914))

## [0.3.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/pulumi-provider-coolify-v0.2.1...pulumi-provider-coolify-v0.3.0) (2026-09-05)


### Features

* native Pulumi provider for Coolify v4 ([3e21b47](https://github.com/bambamboole/pulumi-provider-coolify/commit/3e21b4752250b03b38c89c44723f30418b823d8a))
* support pull request previews and docker tag overrides on Deployment ([c812f4e](https://github.com/bambamboole/pulumi-provider-coolify/commit/c812f4ef2ce254c62c6966575e1e8461d49c4843))

## [0.2.0](https://github.com/bambamboole/pulumi-provider-coolify/compare/pulumi-provider-coolify-v0.1.0...pulumi-provider-coolify-v0.2.0) (2026-09-05)


### Features

* native Pulumi provider for Coolify v4 ([3e21b47](https://github.com/bambamboole/pulumi-provider-coolify/commit/3e21b4752250b03b38c89c44723f30418b823d8a))
* support pull request previews and docker tag overrides on Deployment ([c812f4e](https://github.com/bambamboole/pulumi-provider-coolify/commit/c812f4ef2ce254c62c6966575e1e8461d49c4843))
