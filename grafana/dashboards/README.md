# KEIT Grafana dashboards (reusable)

Four Grafana dashboards developed for KEIT on Scaleway, generalised so they work on any cluster running the relevant exporters. Drop them into your own Grafana to surface carbon hotspots, compare measurement methodologies, and find right-sizing opportunities.

These are the dashboards behind Flavia Paganelli's *KEIT, One Year On: Measuring Kubernetes Carbon on a Sovereign Cloud* talk at [Green IO Amsterdam 2026](https://greenio.tech/conference/25/amsterdam-2026-june).

## What each dashboard does

| Dashboard | Purpose | Required exporters |
|---|---|---|
| **`keit-scaleway-footprint`** | Surface raw Scaleway vendor data (CO₂ + water by project, region, SKU, service category). Daily trends, flight-km equivalents. Good starting point. | `keit/scaleway-footprint-exporter` |
| **`keit-scaleway-vs-sci-comparison`** | Side-by-side: KEIT-computed SCI vs Scaleway-bundled vendor figure. Shows the gap, and lets you decompose it. | `keit/scaleway-footprint-exporter`, Kepler, grid-intensity-exporter |
| **`keit-scaleway-hotspots`** | Finding-oriented: top projects, top SKUs, the regional-grid-effect heatmap (same machine in different zones), regression detection. Practitioner-actionable. | `keit/scaleway-footprint-exporter` |
| **`keit-rightsizing`** | Kepler-relative pod rankings + kube-state-metrics waste detection. Surfaces overprovisioned pods, underprovisioned pods (request > actual), pods missing requests, and 7-day cumulative energy. | Kepler (v0.6.x / 0.8.0 — *not* 0.10+ on managed K8s yet), kube-state-metrics, cadvisor |

## How to install

### Plain Grafana (any version 9+)

1. Open Grafana → Dashboards → Import
2. Upload the `.json` file (or paste its contents)
3. Pick your Prometheus / Mimir datasource when prompted
4. Set the **Cluster** template variable to match your `kubernetes_cluster` external label (or use `.+` to match all)

### Grafana Operator (Kubernetes-native, GitOps)

The `grafana-operator-crds/` directory has the same dashboards wrapped in `GrafanaDashboard` CRDs. Two things to edit before applying:

1. **`metadata.namespace`**: set to your Grafana Operator's installation namespace
2. **`spec.instanceSelector.matchLabels`**: set to whatever labels your `Grafana` CR uses to identify itself (see [grafana-operator docs](https://grafana.github.io/grafana-operator/docs/dashboards/))

Then `kubectl apply -f` and the operator will create the dashboards in your Grafana instance.

## Required metrics

The dashboards expect these metrics to be in your Prometheus / Mimir:

### From `keit/scaleway-footprint-exporter`
- `keit_scaleway_co2_kg{report_date, project_id, project_name, region, zone, sku, service_category, product_category}`
- `keit_scaleway_water_m3{...same labels...}`
- `keit_scaleway_data_lag_days`
- `keit_scaleway_scrape_success`

### From Kepler (pre-0.10 rewrite — v0.8.0 / chart 0.6.1)
- `kepler_container_joules_total{container_namespace, pod_name, mode, ...}`
- `kepler_node_platform_joules_total{...}`

⚠️ Kepler v0.10+ dropped this metric and currently doesn't support VMs. If you're on managed K8s (EKS, GKE, AKS, Kapsule, etc.), stay on chart 0.6.1 / binary 0.8.0 until upstream restores VM support. See [issue #2363](https://github.com/sustainable-computing-io/kepler/issues/2363).

### From grid-intensity-go (`v0.7.0`)
- `grid_intensity_carbon_average{location, region, provider, ...}`

If you don't have an Aknostic-style internal mirror image, you can use `ghcr.io/aknostic/grid-intensity:v0.7.0` (public mirror — upstream doesn't publish a container image).

### From kube-state-metrics + cadvisor
- `kube_pod_container_resource_requests{namespace, pod, container, resource}`
- `container_cpu_usage_seconds_total{namespace, pod, container}`
- `container_memory_working_set_bytes{namespace, pod, container}`
- `kube_pod_container_info{namespace, pod, container}`

These come for free with most Kubernetes monitoring setups (Prometheus Operator, Grafana Alloy, etc.).

## Cluster-label assumption

The dashboards filter by `kubernetes_cluster=~"$cluster"`. This assumes your Prometheus scrape adds a `kubernetes_cluster` external label to all metrics — common for Alloy / Mimir multi-cluster setups.

If your label is named differently (`cluster`, `cluster_name`, etc.), do a find-and-replace in the JSON before importing. Or set the `cluster` variable to `.+` to match everything.

## Adjusting for your setup

A few hardcoded constants live in the `keit-scaleway-vs-sci-comparison` dashboard. Edit these template variables to match your environment:

| Variable | Default | What to change it to |
|---|---|---|
| `PUE_scaleway` | `1.38` | Your DC's PUE (look up at your cloud provider's sustainability page) |
| `embodied_kg_5yr` | `195` | Boavizta-derived embodied for your cluster: `(per-node-yearly-kg) × (node count) × (lifetime years)` |
| `seconds_five_years` | `157766400` | If you use a different amortization lifetime |

For embodied lookup: `POST https://api.boavizta.org/v1/cloud/instance?duration=8760` with `{"provider":"<your-provider>","instance_type":"<sku>"}`.

## Source of truth

These dashboards are also deployed in production via [aknostic/groupware](https://github.com/aknostic/groupware) (private — Aknostic's internal infra), under `heystaq-config/dashboards/`. The copies here are the **public, generalised versions**. Changes to either form should be kept in sync.

## License

These dashboards inherit KEIT's license (see repository root). Free to use, modify, and re-publish.
