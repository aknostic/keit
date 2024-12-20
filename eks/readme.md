# eks

## create AWS EKS cluster

To create the EKS cluster, from the commandline:

```
eksctl create cluster -f cluster-config.yaml
```

***

## Deploy KEIT:
```
helm install --namespace keit -f eks/helm/values.yaml keit-boavizta-exporter eks/helm --create-namespace
```
***

## Usage, examples:

Grafana:

```
kubectl -n prometheus port-forward svc/prometheus-grafana 3000:80 &
```

[http://127.0.0.1:3000](http://127.0.0.1:3000)

Check in browser, username, password. Look for the dashboard **Carbon intensity EKS - KEIT (Ierland)** if not there import the file KEIT\_grafana\_dashboard.json into grafana.

Prometheus:

```
kubectl -n prometheus port-forward svc/prometheus-operated 9090 &
```

[http://127.0.0.1:9090](http://127.0.0.1:9090)

check the keit metrics, embodied/embedded value of instances.

```
kubectl -n keit port-forward service/keit-service 8080:8080 &
curl -s http://localhost:8080/metrics
```

***

## Keit

![keit](images/keit.png)

## grid-intensity

![monitoring](images/monitoring.png)

## Kepler

![kepler](images/kepler.png)

## prometheus

![prometheus](images/prometheus.png)
