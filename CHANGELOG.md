
<a name="v0.2.0"></a>
## [v0.2.0](https://github.com/vatesfr/xenorchestra-cloud-controller-manager/compare/v0.1.0...v0.2.0) (2026-01-08)

Welcome to the v0.2.0 release of Kubernetes cloud controller manager for Xen Orchestra!

### Bug Fixes

- workaround for the SystemUUID sometimes in little-endian
- replace deprecated SDK method after version bump
- add missing mocks

### Features

- Add unit tests
- record events when node zone and node region changed

### Changelog

* abb9ff7 fix: workaround for the SystemUUID sometimes in little-endian
* 05f0a4b fix: replace deprecated SDK method after version bump
* 0103ee4 build(deps): bump Xen Orchestra SDK version
* 08ccdc8 refactor: make xoClient and xoConfig available outside package scope
* e857ac0 fix: add missing mocks
* 75057ac build(deps): bump github.com/vatesfr/xenorchestra-go-sdk
* 2089a13 style: fix linting
* 127b9bb feat: Add unit tests
* 8269c73 feat: record events when node zone and node region changed
* d9f95f8 build(deps): bump github.com/spf13/pflag from 1.0.6 to 1.0.7
* 53430f5 build(deps): bump sigstore/cosign-installer from 3.9.1 to 3.9.2
* c50202f build(deps): bump golang from 1.24.4-alpine to 1.24.5-alpine
* 678a332 build(deps): bump the k8s-io group with 5 updates

<a name="v0.1.0"></a>
## [v0.1.0](https://github.com/vatesfr/xenorchestra-cloud-controller-manager/compare/v0.0.4...v0.1.0) (2025-07-29)

Welcome to the v0.1.0 release of Kubernetes cloud controller manager for Xen Orchestra!

### Bug Fixes

- installation doc, fix link to more values

### Features

- add cloud-node-label-sync into chart and deployments
- add cloud-node-label-sync controller to sync labels with actual XO VM state

### Changelog

* 306ab27 chore: release v0.1.0
* dea32aa build: bump chart version
* 09320f1 feat: add cloud-node-label-sync into chart and deployments
* 6389856 feat: add cloud-node-label-sync controller to sync labels with actual XO VM state
* 64598d1 chg: installation doc, using kubectl, rename config to config.yaml, fix command lines.
* 5d3846f fix: installation doc, fix link to more values

<a name="v0.0.4"></a>
## [v0.0.4](https://github.com/vatesfr/xenorchestra-cloud-controller-manager/compare/v0.0.3...v0.0.4) (2025-06-26)

Welcome to the v0.0.4 release of Kubernetes cloud controller manager for Xen Orchestra!

### Bug Fixes

- cloud-node-lifecycle and improve tests

### Features

- **chart:** Add hostnetwork value  for the CCM deployment

### Changelog

* 50039dd chore: release v0.0.4
* 7a3804a build(chart): bump version
* 3a7b6a8 feat(chart): Add hostnetwork value  for the CCM deployment
* 9b25da1 build(lint): Fix yaml linting and add make cmd
* 8415b9e fix: cloud-node-lifecycle and improve tests
* b31eabc docs: Update installation method with Helm chart.

<a name="v0.0.3"></a>
## [v0.0.3](https://github.com/vatesfr/xenorchestra-cloud-controller-manager/compare/v0.0.2...v0.0.3) (2025-06-24)

Welcome to the v0.0.3 release of Kubernetes cloud controller manager for Xen Orchestra!

### Bug Fixes

- **chart:** wrong config filename

### Changelog

* 6bc1196 chore: release v0.0.3
* 5775779 build(deps): bump golang dependencies
* 77478a3 fix(chart): wrong config filename
* 4a45836 chore: release v0.0.2 (to generate Chart)
* 1a20839 build(deps): bump sigstore/cosign-installer from 3.8.2 to 3.9.1
* 64804a2 build(deps): bump golangci/golangci-lint-action from 7 to 8
* 6e106a8 build(deps): bump golang from 1.24.2-alpine to 1.24.4-alpine

<a name="v0.0.2"></a>
## [v0.0.2](https://github.com/vatesfr/xenorchestra-cloud-controller-manager/compare/v0.0.1...v0.0.2) (2025-06-24)

Welcome to the v0.0.2 release of Kubernetes cloud controller manager for Xen Orchestra!

### Bug Fixes

- add missing mocks and update linter

### Changelog

* e693ab5 chore: update linter
* 121c495 fix: add missing mocks and update linter

<a name="v0.0.1"></a>
## v0.0.1 (2025-06-23)

Welcome to the v0.0.1 release of Kubernetes cloud controller manager for Xen Orchestra!

### Changelog

* bf8c827 ci: add workflow
