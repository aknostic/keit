# scaleway-footprint-exporter Helm chart

Helm chart for the Scaleway Environmental Footprint exporter. See the [parent README](../README.md) for what the exporter does and the metrics it emits.

## Install — quickstart (dev / demo)

```bash
helm install scaleway-footprint-exporter . \
  --namespace keit --create-namespace \
  --set secret.scwOrgId="<your-org-uuid>" \
  --set secret.scwSecretKey="<your-api-secret-key>"
```

The chart creates the Secret with the values you pass. **Do not use this pattern in production** — the credentials end up in the Helm release object.

## Install — production (Secret managed out-of-band)

Recommended for any real deployment. Create the Secret with SOPS, sealed-secrets, external-secrets, or whatever your cluster uses, then point the chart at it:

```bash
# however your cluster manages secrets — example with raw kubectl:
kubectl -n keit create secret generic scaleway-footprint-credentials \
  --from-literal=SCW_ORG_ID="$SCW_ORG_ID" \
  --from-literal=SCW_SECRET_KEY="$SCW_SECRET_KEY"

helm install scaleway-footprint-exporter . \
  --namespace keit --create-namespace \
  --set secret.create=false \
  --set secret.existingSecretName=scaleway-footprint-credentials
```

## Install — Flux HelmRelease

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: scaleway-footprint-exporter
  namespace: keit
spec:
  interval: 30m
  chart:
    spec:
      chart: scaleway-footprint-exporter
      sourceRef:
        kind: GitRepository      # or HelmRepository if you publish the chart
        name: keit
        namespace: flux-system
  values:
    image:
      repository: ghcr.io/aknostic/scaleway-footprint-exporter
      tag: v0.1.0
    secret:
      create: false
      existingSecretName: scaleway-footprint-credentials
    scrapeAnnotations:
      enabled: true        # Alloy / Vector / classic Prometheus
    servicemonitor:
      enabled: false       # set true if you run Prometheus Operator
```

The `scaleway-footprint-credentials` Secret should be defined separately in your GitOps repo, encrypted with your cluster's SOPS key.

## Values reference

See [values.yaml](values.yaml) for inline documentation. The toggles you'll touch most:

| Key | Default | Notes |
|-----|---------|-------|
| `image.repository` | `ghcr.io/aknostic/scaleway-footprint-exporter` | Container image |
| `image.tag` | `""` | Empty → uses `Chart.appVersion` |
| `scrapeAnnotations.enabled` | `true` | `prometheus.io/scrape` annotations on pod (Alloy / Vector / classic Prom) |
| `servicemonitor.enabled` | `false` | Enable for Prometheus Operator clusters |
| `secret.create` | `true` | Set `false` + `existingSecretName` for production |
| `secret.existingSecretName` | `""` | Name of pre-created Secret with `SCW_ORG_ID` + `SCW_SECRET_KEY` |

## Verify

```bash
kubectl -n keit logs -l app=scaleway-footprint-exporter -f
kubectl -n keit port-forward svc/scaleway-footprint-exporter 8080:8080
curl -s localhost:8080/metrics | grep keit_scaleway | head
```
