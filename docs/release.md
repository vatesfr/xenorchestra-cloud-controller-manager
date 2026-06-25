# Release

Each release publishes the application image and Helm chart from one Git tag:

```text
vAPP_VERSION-CHART_INCREMENT
```

The chart version remains independent. Before creating the tag:

1. Bump `version` and set `appVersion` to `vAPP_VERSION` in `Chart.yaml`.
2. Generate the release files.

```shell
TAG=vAPP_VERSION make docs
RELEASE_TAG=vAPP_VERSION-CHART_INCREMENT make release-update
```

3. Merge the changes into `main`, then create and push the tag.

```shell
git tag vAPP_VERSION-CHART_INCREMENT
git push origin vAPP_VERSION-CHART_INCREMENT
```

The Git tag identifies the release. The application image and the chart's `appVersion` use `vAPP_VERSION`.
