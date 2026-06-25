# Verify images

We'll be employing [Cosign's](https://github.com/sigstore/cosign) keyless verifications to ensure that images were built in Github Actions.

## Verify Helm chart

We will verify the keyless signature using the Cosign protocol.

```shell
cosign verify ghcr.io/vatesfr/charts/xenorchestra-cloud-controller-manager:CHART_VERSION --certificate-identity-regexp 'https://github.com/vatesfr/xenorchestra-cloud-controller-manager/.github/workflows/release.yaml@refs/tags/v.*' --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Verify containers

We will verify the keyless signature using the Cosign protocol.

```shell
# Edge version
cosign verify ghcr.io/vatesfr/xenorchestra-cloud-controller-manager:edge --certificate-identity https://github.com/vatesfr/xenorchestra-cloud-controller-manager/.github/workflows/build-edge.yaml@refs/heads/main --certificate-oidc-issuer https://token.actions.githubusercontent.com

# Releases
cosign verify ghcr.io/vatesfr/xenorchestra-cloud-controller-manager:vAPP_VERSION --certificate-identity-regexp 'https://github.com/vatesfr/xenorchestra-cloud-controller-manager/.github/workflows/release.yaml@refs/tags/v.*' --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
