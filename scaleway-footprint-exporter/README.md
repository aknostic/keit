# scaleway-footprint-exporter

Prometheus exporter for the Scaleway Environmental Footprint API. Polls daily-aggregate carbon and water data per Scaleway project / region / zone / SKU and exposes them as Prometheus metrics.

## Why this exists

KEIT historically uses Kepler for energy and Boavizta for embodied carbon. Neither works well on Scaleway Kapsule (managed K8s, VMs without RAPL). This exporter adds Scaleway's native ADEME-based footprint data as a third path — works on any Scaleway service (Instances, Block Storage, Object Storage, Kapsule control plane, etc.), with both carbon and water.

## What it emits

Per-day SKU gauges plus two scrape-health gauges:

```
keit_scaleway_co2_kg{report_date, project_id, project_name, region, zone, sku, service_category, product_category}
keit_scaleway_water_m3{report_date, project_id, project_name, region, zone, sku, service_category, product_category}
keit_scaleway_data_lag_days
keit_scaleway_scrape_success
```

Each scrape (hourly) re-queries the trailing 14 UTC days as 14 separate 1-day calls. Every day with non-empty data is emitted with `report_date=YYYY-MM-DD` (the start of the 1-day window). This produces a real daily timeseries for Grafana — `sum by (report_date)(keit_scaleway_co2_kg)` plots one bar per day.

`keit_scaleway_data_lag_days` is the whole UTC days between yesterday and the most recent day with data; 0 means yesterday is fresh, `lookbackDays` (14) means nothing in the window has data. `keit_scaleway_scrape_success` is 1 if at least one day's fetch succeeded in the most recent scrape, else 0 — when 0, the per-day gauges retain their last-known-good values.

## Granularity caveats — read these

- **SKU is a resource type, not an instance.** A single value covers *all* PRO2-XXS instances in fr-par-1 in that project. Per-instance / per-pod attribution is not possible from this data alone — you have to join with Kubernetes metadata and pick an attribution rule (CPU-weighted, memory-weighted, even split). Do that in a Prometheus recording rule, not here.
- **Data lag is real.** Scaleway's footprint pipeline can lag several days; `keit_scaleway_data_lag_days` exposes how far behind it is. Alert on it crossing some threshold (e.g. > 5).
- **Some regions report `m3_water_usage = 0`** (e.g. WAW). Scaleway hasn't published water data for every datacenter.
- **Kapsule control plane** (`/k8s/control-plane/<region>`) is a flat ~0.14 kg per project per month in fr-par; doesn't scale with workload.

## Configuration

Two required env vars:

| Var               | What                                                  |
|-------------------|-------------------------------------------------------|
| `SCW_ORG_ID`      | Scaleway Organization ID                              |
| `SCW_SECRET_KEY`  | API key from an IAM Application with footprint:read   |

## Run locally

```bash
export SCW_ORG_ID=...
export SCW_SECRET_KEY=...
go mod tidy
go run .
# in another terminal:
curl localhost:8080/metrics | grep keit_scaleway
```

## Build container

```bash
docker build -t scaleway-footprint-exporter .
docker run -p 8080:8080 -e SCW_ORG_ID -e SCW_SECRET_KEY scaleway-footprint-exporter
```

## Releasing (publishing a new image)

CI publishes the image; nobody needs to `docker push` by hand.

1. Bump `version` and `appVersion` in [`helm/Chart.yaml`](helm/Chart.yaml) to the new version, e.g. `0.2.1`.
2. Push a tag named `scaleway-footprint-exporter-v<version>`, e.g.:
   ```bash
   git tag scaleway-footprint-exporter-v0.2.1
   git push origin scaleway-footprint-exporter-v0.2.1
   ```
3. The `Build and Publish scaleway-footprint-exporter` GitHub Actions workflow builds the image from this directory's `Dockerfile` (`linux/amd64`, matching the Dockerfile's hardcoded `GOARCH=amd64`) and pushes `ghcr.io/aknostic/scaleway-footprint-exporter:<version>` to GHCR. It refuses to overwrite a tag that already exists.
4. The workflow can also be run manually (`workflow_dispatch`) with a `tag` input, for rebuilding a version without pushing a new git tag.

## Deploy to Kubernetes

Two paths are supported — pick whichever matches your tooling:

- **Helm chart** (recommended) → [`helm/`](helm/README.md). Idiomatic install, parameterized via `values.yaml`, plays well with Flux `HelmRelease` and ArgoCD `Application`. CI publishes `ghcr.io/aknostic/scaleway-footprint-exporter` (see [Releasing](#releasing-publishing-a-new-image)) and the chart's `values.yaml` already points at it, defaulting the tag to `.Chart.AppVersion` — no build step required, just `helm install`.
- **Raw manifests** → [`manifests/`](#raw-manifests). For users on plain `kubectl apply` or who prefer to vendor and patch the YAML themselves. The published image works here too; only build your own if you need to vendor a modified image.

Both routes support annotation-based scrape discovery (Alloy / Vector / classic Prometheus) and Prometheus Operator's `ServiceMonitor`.

### Raw manifests

All K8s manifests live in `manifests/`.

### Files

- `manifests/namespace.yml` — creates the `keit` namespace
- `manifests/secret.yml` — **template** for `SCW_ORG_ID` + `SCW_SECRET_KEY`. Replace placeholders or use SOPS / sealed-secrets / external-secrets per your cluster's conventions before applying.
- `manifests/deployment.yml` — runs the exporter; pulls credentials from the Secret. Includes `prometheus.io/scrape` annotations for annotation-based discovery.
- `manifests/service.yml` — ClusterIP `Service` exposing port `metrics`/8080. Required by ServiceMonitor; useful for `kubectl port-forward` debugging.
- `manifests/servicemonitor.yml` — Prometheus Operator `ServiceMonitor`. Apply if you have the operator installed.

### Quick deploy

```bash
export KUBECONFIG=/path/to/your/kubeconfig

# 1. manifests/deployment.yml ships with a REPLACE_WITH_REGISTRY placeholder
#    image, for people who intend to vendor/patch the raw manifests with
#    their own build. To use the published image instead, edit the `image:`
#    line in manifests/deployment.yml and point it at
#    ghcr.io/aknostic/scaleway-footprint-exporter:0.2.1 (no build/push
#    needed). Only build & push your own image if you're vendoring a
#    modified one.
#
#    Note: the 0.2.1 tag becomes available on GHCR only after the
#    scaleway-footprint-exporter-v0.2.1 release tag is pushed and its build
#    workflow has run. Until then, use an existing tag such as 0.2.0 or
#    latest.

# 2. Namespace + Secret + Deployment + Service
kubectl apply -f manifests/namespace.yml
kubectl -n keit create secret generic scaleway-footprint-credentials \
  --from-literal=SCW_ORG_ID="$SCW_ORG_ID" \
  --from-literal=SCW_SECRET_KEY="$SCW_SECRET_KEY"
kubectl apply -f manifests/deployment.yml -f manifests/service.yml

# 3. Pick ONE scrape path:
#    (a) Prometheus Operator users — apply the ServiceMonitor:
kubectl apply -f manifests/servicemonitor.yml
#    (b) Alloy / Vector / annotation-based discovery — no extra apply needed;
#        the Deployment already has prometheus.io/scrape annotations.

# 4. Watch logs and confirm metrics
kubectl -n keit logs -l app.kubernetes.io/name=scaleway-footprint-exporter -f
kubectl -n keit port-forward svc/scaleway-footprint-exporter 8080:8080
curl -s localhost:8080/metrics | grep keit_scaleway | head
```

### GitOps (Flux, ArgoCD, etc.)

Point your tooling at the `manifests/` directory — a Flux `Kustomization`, an ArgoCD `Application`, or a HelmRelease wrapping the directory. Encrypt the Secret with SOPS / sealed-secrets / your preferred mechanism — never commit real credentials.

## Known TODOs

- No backfill: missed scrapes lose that day's data point. For most use cases that's fine; if continuity matters, add a startup loop that fetches the last N days.
- No retry/backoff on API errors. Acceptable since data refreshes daily and one missed scrape won't matter.
