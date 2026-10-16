# KEIT - Kubernetes Emissions Insights Tool

KEIT is an open source tool which estimates the carbon emissions of a Kubernetes cluster.

### Support matrix

| Environment | Status | Notes |
| --- | --- | --- |
| Bare metal | ✅ Supported | Install Kepler chart `0.6.1` (see [Install Kepler](#install-kepler)). |
| AWS EKS | ✅ Supported | Install Kepler chart `0.6.1`. Embodied emissions are retrieved dynamically. |
| Scaleway Kapsule | ✅ Supported | Via the [Scaleway footprint exporter](scaleway-footprint-exporter). |
| Azure AKS | 🚧 In progress | |
| Google GKE | 📅 Planned | |

On managed Kubernetes, nodes are virtual machines. Kepler v0.10 and later cannot report power inside a VM, so KEIT needs Kepler chart `0.6.1` there. [Install Kepler](#install-kepler) explains why.

KEIT uses other open source projects to accomplish this:

- [Kepler](https://github.com/sustainable-computing-io/kepler) for estimating the energy consumption of the running software.
- [ElectricityMaps](https://app.electricitymaps.com/map/24h) to get the energy carbon intensity of the electricity consumed.
- [grid-intensity-go](https://github.com/thegreenwebfoundation/grid-intensity-go)'s Prometheus exporter to get ElectricityMaps data.
- [Boavizta API](https://doc.api.boavizta.org/) to get data about the embodied emissions of the hardware being used.
- [Prometheus](https://prometheus.io/) to collect and store all this data.
- [Grafana](https://grafana.com/) to visualize the data.
- [Metrics Server](https://github.com/kubernetes-sigs/metrics-server) for data about resources utilisation.

This means that to use KEIT you will need to install a few software components.

![KEIT components](/images/keit-components.png)


## Installation

The installation instructions assume you have a Kubernetes cluster up and running and that you can access it via `kubectl` command line.
They illustrate how to install the components manually with [Helm](https://helm.sh/), but you can adapt them to a GitOps method like ArgoCD, Flux, etc. 

### Install Metrics Server

If you don't have the metrics server yet, you can install it via Helm.

Add the metrics-server repo to Helm:

```bash
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
```

After you've installed the repo you can install the chart.

```bash
helm upgrade --install metrics-server metrics-server/metrics-server
```

Check if it's running:

```bash
kubectl get deployment metrics-server -n kube-system
NAME             READY   UP-TO-DATE   AVAILABLE   AGE
metrics-server   1/1     1            1           5d1h
```

and look if we can use it:

```bash
kubectl top nodes
NAME                                           CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%   
ip-192-168-20-71.eu-west-1.compute.internal    118m         6%     1579Mi          48%       
ip-192-168-77-138.eu-west-1.compute.internal   88m          4%     787Mi           23% 
```

### Install Prometheus and Grafana (kube-prometheus-stack)

If you are not already using Prometheus and Grafana, here's how you can install it.

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
kubectl create namespace prometheus
helm install --set prometheus.prometheusSpec.tsdb.outOfOrderTimeWindow=3h prometheus prometheus-community/kube-prometheus-stack -n prometheus
```

#### Note about Prometheus configuration

If you are using the exporter with the ElectricityMaps provider, it will return a value for estimated, which will be the most recent one, and another value for the real value, which can be a few hours in the past. Depending on your Prometheus installation, it could be that the metrics that have a timestamp in the past are not accepted, with an error such as this:

```
Error on ingesting samples that are too old or are too far into the future
```

That's why we configure the property `tsdb.outOfOrderTimeWindow` to extend the time window accepted to `3h`.


You can check that Prometheus is running:

```bash
kubectl port-forward svc/prometheus-operated 9090:9090 -n prometheus
```

and go to http://localhost:9090

And you can check that Grafana is running and check out the default dashboards:

```bash
kubectl port-forward svc/prometheus-grafana 3000:80 -n prometheus
```

Open a browser with the following url:

http://127.0.0.1:3000

The default login is `admin/prom-operator`.


### Install Kepler

Install Kepler chart `0.6.1`, which runs Kepler 0.8.0, on every cluster, bare metal included:

```bash
helm repo add kepler https://sustainable-computing-io.github.io/kepler-helm-chart
helm repo update
helm install kepler kepler/kepler --version 0.6.1 --namespace kepler --create-namespace --set serviceMonitor.enabled=true --set serviceMonitor.labels.release=prometheus
```

#### Why chart 0.6.1

- **Managed Kubernetes (EKS, GKE, AKS, Kapsule).** Since Kepler v0.10, Kepler reports only energy measured through RAPL/powercap (or hwmon). A cloud VM does not expose those counters, so Kepler v0.10 and later gives no power figures there and the dashboards show "No data". Kepler 0.8.0 can still estimate power in a VM, which is what KEIT relies on there. Work on a VM power model for the current Kepler has started: see [kepler#2589](https://github.com/sustainable-computing-io/kepler/issues/2589) and the [draft proposal](https://github.com/sustainable-computing-io/kepler/blob/main/docs/developer/proposal/draft-20260417-VM-CPU-Power-Models-Training.md).
- **Bare metal.** Kepler v0.12 can read RAPL and hwmon on bare metal, but KEIT does not support it yet. KEIT's queries use `kepler_container_joules_total`, which [Kepler documents](https://github.com/sustainable-computing-io/kepler/blob/main/docs/user/metrics.md) as `kepler_container_cpu_joules_total` in current releases (see also [kepler#2363](https://github.com/sustainable-computing-io/kepler/issues/2363)). Until KEIT is updated, stay on chart `0.6.1`.
- **Why `0.6.1` and not a later chart.** The `kepler-helm-chart` repository is deprecated; Kepler now publishes its chart as an OCI artifact from the main Kepler repository. The last chart in the old repository, `0.6.2`, still runs Kepler 0.8.0 and is marked deprecated; the only difference from `0.6.1` is that flag.

You can get the Kepler Grafana dashboard from [here](https://github.com/sustainable-computing-io/kepler/blob/main/grafana-dashboards/Kepler-Exporter.json) and check the data you are getting.

### Install grid-intensity-go exporter

To get data about energy carbon intensity we use ElectricityMaps API, and to get the data to Prometheus we use the Prometheus exporter provided by grid-intensity-go.

We need to tell ElectricityMaps API where we are in the world - where is the data center or hardware where your cluster is running. Currently, we configure it once, it's not picked up dynamically.

For this, we need the id of the zone as listed by ElectricityMaps. You can get the list of zones by calling the /zones endpoint:

```bash
curl https://api.electricitymap.org/v3/zones | jq
{
  "AD": {
    "zoneName": "Andorra"
  },
  "AE": {
    "zoneName": "United Arab Emirates"
  },
  "AF": {
    "zoneName": "Afghanistan"
  },
  "AG": {
    "zoneName": "Antigua and Barbuda"
  },
...
"IE": {
    "zoneName": "Ireland"
  },
...
```

 For example, if you are running AWS on Ireland region, you will use `IE` as zone id.

We will be using the free tier of the ElectricityMaps API, for which you will need to request an API token [here](https://api-portal.electricitymaps.com/).

Replace the `grid-intensity-exporter/values.yml` with the zone and API token obtained.

Install the grid-intensity-exporter helm chart:

```bash
kubectl apply -f keit/grid-intensity-exporter/namespace.yml
kubectl apply -f keit/grid-intensity-exporter/servicemonitor.yml
git clone git@github.com:thegreenwebfoundation/grid-intensity-go.git
helm install -n grid-intensity -f keit/grid-intensity-exporter/values.yml grid-intensity-exporter grid-intensity-go/helm/grid-intensity-exporter
```

### Get embodied emissions

The embodied emissions of the hardware running your cluster can be estimated using [Boavizta](https://boavizta.org/).

If you are running on AWS, KEIT runs the Boavizta API in your cluster to dynamically retrieve the embodied emissions of the instances that you are running. In that way, you can use something like Karpenter or Kubernetes Autoscaler to adjust the size of your cluster dynamically and KEIT will retrieve the embodied emissions accordingly.

#### If you are running on AWS EKS (dynamic)

Install the helm chart:

```bash
helm install --namespace keit -f helm/values.yaml keit-boavizta-exporter helm --create-namespace
```

And you can test that the Boavizta exporter is running:
```bash
kubectl port-forward svc/keit-service 8080:8080 -n keit &
curl localhost:8080/metrics | grep embedded
# HELP eks_node_embedded_value The embedded value of AWS instance types running in the EKS cluster.
# TYPE eks_node_embedded_value gauge
eks_node_embedded_value{instance_type="c5.4xlarge",node_name="ip-10-12-48-197.eu-west-1.compute.internal"} 130
eks_node_embedded_value{instance_type="c5.4xlarge",node_name="ip-10-12-58-56.eu-west-1.compute.internal"} 130
eks_node_embedded_value{instance_type="c5.large",node_name="ip-10-12-68-90.eu-west-1.compute.internal"} 16
eks_node_embedded_value{instance_type="m5a.large",node_name="ip-10-12-16-152.eu-west-1.compute.internal"} 19
...
```

#### If you are running anywhere else (static)

We have plans to implement the same logic as above for Azure, but for the time being the solution on other cloud providers or data centers is to calculate the embodied emissions using [Datavizta](https://datavizta.boavizta.org/serversimpact), entering the data about your hardware.

Note: We want like to add support to Azure and maybe GCP.

### Create the Grafana dashboard

#### Find or estimate the PUE of the data center

The [PUE](https://www.cloudcarbonfootprint.org/docs/methodology/#power-usage-effectiveness) (Power Usage Effectiveness) of a data center or cloud provider is a score of how energy efficient a data center is, with the lowest possible score of 1 meaning all energy consumed goes directly to powering the servers and none is being wasted on cooling.

Find the PUE of your cloud provider or data center publicly available, or ask the number to your data center.

For example:
- For example, [AWS](https://sustainability.aboutamazon.com/products-services/aws-cloud) in Ireland reports a very low PUE of 1.10
- For example, [GCP](https://www.google.com/about/datacenters/efficiency/) in Ireland reports even lower PUE of 1.08
- [Azure](https://azure.microsoft.com/en-us/blog/how-microsoft-measures-datacenter-water-and-energy-use-to-improve-azure-cloud-sustainability/) reports a PUE of 1.185 in Europe
- [Scaleway](https://www.scaleway.com/en/environmental-leadership/) reports a PUE of 1.37
- And so on, look for your provider published value on PUE, of if not available ask them for it.

#### Create the Grafana dashboard

The KEIT Grafana dashboard definition is in the repository, grafana/KEIT_grafana_dashboard.json.

Update the PUE to your value and then import it to Grafana.

You can access Grafana by port forwarding:

```bash
kubectl port-forward svc/kube-prometheus-stack-grafana 8082:80 -n monitoring &
```
And view your KEIT dashboard locally at http://localhost:8082 (with the default credentials admin/prom-operator, if you just installed it).

#### Additional reusable dashboards

The `grafana/dashboards/` directory contains four more dashboards developed for KEIT on Scaleway, genericised to work on any cluster:

- **`keit-scaleway-footprint`** — raw vendor data from the Scaleway Environmental Footprint API
- **`keit-scaleway-vs-sci-comparison`** — side-by-side KEIT-computed SCI vs Scaleway-bundled
- **`keit-scaleway-hotspots`** — finding-oriented (top SKUs, regional-grid heatmap, regression detection)
- **`keit-rightsizing`** — Kepler + kube-state-metrics for overprovisioned / underprovisioned pods

See [`grafana/dashboards/README.md`](grafana/dashboards/README.md) for what each does, required metrics, and how to import them.

