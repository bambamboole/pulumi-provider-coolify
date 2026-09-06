# Changelog

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
