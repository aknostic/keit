# scaleway-footprint-exporter

Prometheus exporter for the Scaleway Environmental Footprint API. Polls daily-aggregate carbon and water data per Scaleway project / region / zone / SKU and exposes them as Prometheus metrics.

## Why this exists

KEIT historically uses Kepler for energy and Boavizta for embodied carbon. Neither works well on Scaleway Kapsule (managed K8s, VMs without RAPL). This exporter adds Scaleway's native ADEME-based footprint data as a third path — works on any Scaleway service (Instances, Block Storage, Object Storage, Kapsule control plane, etc.), with both carbon and water.

## What it emits

Two metric families, both gauges, both labeled the same:

```
keit_scaleway_co2_kg{project_id, project_name, region, zone, sku, service_category, product_category}
keit_scaleway_water_m3{project_id, project_name, region, zone, sku, service_category, product_category}
```

Each metric value is the **previous full UTC day's total** for that combination. Re-emitted every hour (the underlying API only refreshes daily).

## Granularity caveats — read these

- **SKU is a resource type, not an instance.** A single value covers *all* PRO2-XXS instances in fr-par-1 in that project. Per-instance / per-pod attribution is not possible from this data alone — you have to join with Kubernetes metadata and pick an attribution rule (CPU-weighted, memory-weighted, even split). Do that in a Prometheus recording rule, not here.
- **Daily aggregate, not real-time.** "Yesterday" is the freshest you'll get.
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

## Deploy to Kubernetes

Two paths are supported — pick whichever matches your tooling:

- **Helm chart** (recommended) → [`helm/`](helm/README.md). Idiomatic install, parameterized via `values.yaml`, plays well with Flux `HelmRelease` and ArgoCD `Application`.
- **Raw manifests** → [`manifests/`](#raw-manifests). For users on plain `kubectl apply` or who prefer to vendor and patch the YAML themselves.

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

# 1. Build & push the image to a registry the cluster can pull from
docker build -t <your-registry>/scaleway-footprint-exporter:dev .
docker push  <your-registry>/scaleway-footprint-exporter:dev
# Then update the image: line in manifests/deployment.yml.

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
