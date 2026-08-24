# Release

Each release is triggered by a Git tag and publishes the application image and/or the Helm chart from the tagged commit.

## The three versions

| Element | Format | Where |
|---------|--------|-------|
| Application image | `vX.Y.Z` | Docker tag, value of `appVersion` |
| Chart | `X.Y.Z` (no `v` prefix) | `version` in `Chart.yaml`, independent of the app |
| Release tag | `vAPP[-N]` | Git tag that triggers the CI; `N` is the number of chart-only releases for that app version |

The chart version is **decoupled** from the app version: it follows its own semver. The chart's `appVersion` is the version of the application deployed by default (`image.tag` is empty by default, so the deployment uses `appVersion` as the image tag; users can override with `--set image.tag=...`).

## Chart version bump

The chart `version` reflects changes to the chart itself:

| Situation | Bump |
|-----------|------|
| App release, only `appVersion` changes | patch |
| Chart fix (values, templates) | patch |
| Backward-compatible chart feature (new value, new option) | minor |
| Breaking chart change (renamed value, changed default) | major |

Rules:

- An app release always bumps the chart by a **patch**, even if the app itself is a minor or major release. The app version change is carried by `appVersion` and the image tag; if you want to control the app version, pin it with `image.tag`.
- The `version` bump happens in the release commit (the one carrying the tag), not in every pull request. Several pull requests can accumulate chart changes; the last one before tagging applies the bump according to the table. If the bump is forgotten, the OCI registry rejects the duplicate chart version at release time.

## Case 1: App release

The application code changes, with or without chart changes.

1. In `Chart.yaml`, set `appVersion` to the new app version `vX.Y.Z`. Bump `version`: patch if the chart is otherwise unchanged, otherwise according to the table above.
2. Generate the release files:

   ```shell
   TAG=vX.Y.Z make docs
   RELEASE_TAG=vX.Y.Z make release-update
   ```

3. Merge into `main`, then create and push the tag:

   ```shell
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

4. The CI builds and pushes the image `vX.Y.Z`, signs it, and publishes the chart.

> Example: app is at `v1.1.1`, chart `version` is `1.2.0`. A bugfix app release `v1.1.2` with no chart change → `appVersion: v1.1.2`, `version: 1.2.1`, tag `v1.1.2`.
>
> Example: a new app release `v1.2.0` that also adds a new chart value → `appVersion: v1.2.0`, `version: 1.3.0` (feature = minor), tag `v1.2.0`.

## Case 2: Chart-only release

Only `charts/**` changes, the application is unchanged.

1. In `Chart.yaml`, bump `version` according to the table above. Leave `appVersion` unchanged.
2. Regenerate the chart documentation:

   ```shell
   make docs
   ```

3. Merge into `main`, then tag with `N`, the next unused increment for the current app version:

   ```shell
   git tag v<app>-N
   git push origin v<app>-N
   ```

4. The CI publishes the chart only; the image is not rebuilt.

Conditions:

- The app image `v<app>` must already have been released.
- The pull request must not contain application code changes.
- If a chart change is pending when you prepare an app release, fold it into the app release (case 1) instead of tagging a chart-only release first.

> Example (current state): app `v1.1.1` is already released, the chart `version` is `1.2.0` (a new feature, `rbac.create`, is pending). Tag `v1.1.1-1` publishes chart `1.2.0` without touching the image.
>
> Example: after the app release `v1.2.0` (case 1), a chart bugfix → bump `version`, tag `v1.2.0-1`.
