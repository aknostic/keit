# KEIT - Kubernetes Emissions Insights Tool

KEIT is an open source tool which estimates the carbon emissions of a Kubernetes cluster. We have tested it on Kubernetes on bare metal and adapted to EKS and we are working on adapting it to work on Azure.

KEIT uses other open source projects to accomplish this:

- [Kepler](https://github.com/sustainable-computing-io/kepler) for estimating the energy consumption of the running software.
- [ElectricityMaps](https://app.electricitymaps.com/map/24h) to get the energy carbon intensity of the electricity consumed.
- [grid-intensity-go](https://github.com/thegreenwebfoundation/grid-intensity-go)'s Prometheus exporter to get ElectriciyMaps data.
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

and go to http://localhost:9093

And you can check that Grafana is running and check out the default dashboards:

```bash
kubectl port-forward svc/prometheus-grafana 3000:80 -n prometheus
```

Open a browser with the following url:

http://127.0.0.1:3000

The default login is `admin/prom-operator`.


### Install Kepler

```bash
helm repo add kepler https://sustainable-computing-io.github.io/kepler-helm-chart
helm repo update
helm install kepler kepler/kepler --namespace kepler --create-namespace --set serviceMonitor.enabled=true --set serviceMonitor.labels.release=prometheus
```

You can get the kepler grafan dashboard from [here](https://github.com/sustainable-computing-io/kepler/blob/main/grafana-dashboards/Kepler-Exporter.json) and check the data you are getting.

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
helm install -n grid-intensity -f keit/grid-intensity-exporter/values.yaml grid-intensity-exporter grid-intensity-go/helm/grid-intensity-exporter
```

### Find or estimate the PUE of the data center

The [PUE](https://www.cloudcarbonfootprint.org/docs/methodology/#power-usage-effectiveness) (Power Usage Effectiveness) of a data center or cloud provider is a score of how energy efficient a data center is, with the lowest possible score of 1 meaning all energy consumed goes directly to powering the servers and none is being wasted on cooling.

Find the PUE of your cloud provider or data center publicly available, or ask the number to your data center.

For example:
- For example, [AWS](https://sustainability.aboutamazon.com/products-services/aws-cloud) in Ireland reports a very low PUE of 1.10
- For example, [GCP](https://www.google.com/about/datacenters/efficiency/) in Ireland reports even lower PUE of 1.08
- [Azure](https://azure.microsoft.com/en-us/blog/how-microsoft-measures-datacenter-water-and-energy-use-to-improve-azure-cloud-sustainability/) reports a PUE of 1.185 in Europe
- Scaleway reports a PUE of 1.37
- And so on...


### Get embodied emissions

The embodied emissions of the hardware running your cluster can be estimated using [Boavizta](https://boavizta.org/).

If you are running on AWS, KEIT runs the Boavizta API in your cluster to dynamically retrieve the embodied emissions of the instances that you are running. In that way, you can use something like Karpenter or Kubernetes Autoscaler to adjust the size of your cluster dynamically and KEIT will retrieve the embodied emissions accordingly.

#### Running on AWS EKS (dynamic)



#### Running everywhere else (static)

We have plans to implement the same logic as above for Azure, but for the time being the solution on other cloud providers or data centers is to calculate the embodied emissions using [Datavizta](https://datavizta.boavizta.org/serversimpact), entering the data about your hardware.

