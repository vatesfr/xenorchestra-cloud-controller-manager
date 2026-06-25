# Release

Each release publishes the application image and Helm chart from one Git tag:

```text
vAPP_VERSION
vAPP_VERSION-CHART_INCREMENT
```

The first tag for an application version omits the `-0` suffix. The `-CHART_INCREMENT` suffix is only used when the chart changes without changing the application image. If the application image changes, bump `APP_VERSION` instead.

For example, use `v1.2.3` for the first release of that application version, then `v1.2.3-1` if only the chart changes while reusing the `v1.2.3` image.

The chart version remains independent. Before creating the tag:

1. Bump `version` and set `appVersion` to `vAPP_VERSION` in `Chart.yaml`.
2. Generate the release files.

```shell
TAG=vAPP_VERSION make docs
RELEASE_TAG=vAPP_VERSION make release-update
```

For a chart-only release using the same application image, use `RELEASE_TAG=vAPP_VERSION-CHART_INCREMENT` instead. Start the chart increment at `1`.

3. Merge the changes into `main`, then create and push the tag.

```shell
git tag vAPP_VERSION
git push origin vAPP_VERSION
```

The Git tag identifies the release. The application image and the chart's `appVersion` use `vAPP_VERSION`.
