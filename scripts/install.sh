#!/usr/bin/env bash
# One-command KEIT install: Kepler, grid-intensity exporter, the KEIT dashboard
# and (when missing) metrics-server, kube-prometheus-stack and the Boavizta exporter.
# Run it from a clone of the repository. See "Quick install" in the README.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Pinned versions. Kepler 0.6.1 / 0.8.0 is the last release family that works on
# managed Kubernetes and exposes kepler_container_joules_total (see grafana/dashboards/README.md).
KEPLER_CHART_VERSION="0.6.1"
GRID_INTENSITY_TAG="v0.7.0"
GRID_INTENSITY_IMAGE="ghcr.io/aknostic/grid-intensity"

TOKEN="${ELECTRICITYMAPS_TOKEN:-}"
ZONE=""
PUE=""
PROVIDER=""
PROM_NAMESPACE="prometheus"
PROM_RELEASE="prometheus"
DRY_RUN=0

usage() {
  cat <<USAGE
Usage: scripts/install.sh --zone ZONE (--pue PUE | --provider NAME) [--token TOKEN] [options]

Required:
  --zone ZONE           Electricity Maps zone of your cluster, for example IE or DE.
  --pue PUE             PUE of your data center, for example 1.2.
  --provider NAME       Use the published PUE of aws, gcp, azure or scaleway instead of --pue.
  --token TOKEN         Electricity Maps API token. Or set ELECTRICITYMAPS_TOKEN
                        (preferred: keeps it out of your shell history).

Options:
  --prometheus-namespace NS   Namespace of kube-prometheus-stack (default: prometheus).
  --prometheus-release NAME   Helm release name of kube-prometheus-stack (default: prometheus).
                              ServiceMonitors get the label release=NAME.
  --dry-run                   Print the commands that change the cluster, do not run them.
  -h, --help                  Show this help.

--provider aws also installs the Boavizta exporter, which only works on EKS.
USAGE
}

die() { echo "error: $*" >&2; exit 1; }
log() { echo "==> $*"; }

while [ $# -gt 0 ]; do
  case "$1" in
    --token) TOKEN="${2:?--token needs a value}"; shift 2 ;;
    --zone) ZONE="${2:?--zone needs a value}"; shift 2 ;;
    --pue) PUE="${2:?--pue needs a value}"; shift 2 ;;
    --provider) PROVIDER="${2:?--provider needs a value}"; shift 2 ;;
    --prometheus-namespace) PROM_NAMESPACE="${2:?}"; shift 2 ;;
    --prometheus-release) PROM_RELEASE="${2:?}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown option: $1" ;;
  esac
done

[ -n "$TOKEN" ] || die "Electricity Maps token missing: pass --token or set ELECTRICITYMAPS_TOKEN"
[ -n "$ZONE" ] || die "--zone is required (list: curl https://api.electricitymap.org/v3/zones)"

# PUE values published by each provider, as listed in the README.
if [ -z "$PUE" ]; then
  case "$PROVIDER" in
    aws) PUE="1.10" ;;
    gcp) PUE="1.08" ;;
    azure) PUE="1.185" ;;
    scaleway) PUE="1.37" ;;
    "") die "give --pue, or --provider aws|gcp|azure|scaleway" ;;
    *) die "no published PUE known for provider '$PROVIDER': give --pue" ;;
  esac
fi
[[ "$PUE" =~ ^[0-9]+(\.[0-9]+)?$ ]] || die "--pue must be a number such as 1.2, got '$PUE'"
awk -v p="$PUE" 'BEGIN { exit !(p >= 1) }' || die "--pue cannot be below 1"
[[ "$ZONE" =~ ^[A-Za-z0-9_-]+$ ]] || die "--zone looks wrong: '$ZONE'"

for tool in kubectl helm git; do
  command -v "$tool" >/dev/null || die "$tool is required but not on PATH"
done

# Run a command that changes the cluster, or print it with --dry-run.
run() {
  if [ "$DRY_RUN" -eq 1 ]; then echo "[dry-run] $*"; else "$@"; fi
}

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

kubectl cluster-info >/dev/null 2>&1 || die "cannot reach a cluster with kubectl (check your current context)"
log "Cluster: $(kubectl config current-context)"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null
helm repo add kepler https://sustainable-computing-io.github.io/kepler-helm-chart >/dev/null
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/ >/dev/null
helm repo update >/dev/null

# 1. metrics-server, only if the metrics API is not served yet.
if kubectl get apiservice v1beta1.metrics.k8s.io >/dev/null 2>&1; then
  log "metrics-server already present, skipping"
else
  log "Installing metrics-server"
  run helm upgrade --install metrics-server metrics-server/metrics-server
fi

# 2. kube-prometheus-stack, only if the Prometheus Operator CRDs are missing.
if kubectl get crd prometheuses.monitoring.coreos.com >/dev/null 2>&1; then
  log "Prometheus Operator found, not touching your Prometheus."
  cat >&2 <<NOTE
    KEIT needs Prometheus to accept samples up to 3 hours old. Check that your
    Prometheus has tsdb.outOfOrderTimeWindow set to 3h or more; with
    kube-prometheus-stack that is
      prometheus.prometheusSpec.tsdb.outOfOrderTimeWindow=3h
    Without it, Electricity Maps data is rejected as "too old". See the README.
    ServiceMonitors below carry the label release=${PROM_RELEASE}; your Prometheus must select them.
NOTE
else
  log "Installing kube-prometheus-stack in namespace ${PROM_NAMESPACE}"
  run helm upgrade --install "$PROM_RELEASE" prometheus-community/kube-prometheus-stack \
    --namespace "$PROM_NAMESPACE" --create-namespace \
    --set prometheus.prometheusSpec.tsdb.outOfOrderTimeWindow=3h
fi

# 3. Kepler.
log "Installing Kepler ${KEPLER_CHART_VERSION}"
run helm upgrade --install kepler kepler/kepler --version "$KEPLER_CHART_VERSION" \
  --namespace kepler --create-namespace \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.labels.release="$PROM_RELEASE"

# 4. grid-intensity exporter. Upstream publishes the chart in git only.
log "Installing grid-intensity exporter ${GRID_INTENSITY_TAG} for zone ${ZONE}"
git clone --quiet --depth 1 --branch "$GRID_INTENSITY_TAG" \
  https://github.com/thegreenwebfoundation/grid-intensity-go "$WORKDIR/grid-intensity-go"
# Values file with mode 600 so the token never shows in the process list.
umask 077
cat > "$WORKDIR/grid-intensity-values.yaml" <<VALUES
image:
  repository: ${GRID_INTENSITY_IMAGE}
  tag: ${GRID_INTENSITY_TAG}
gridIntensity:
  provider: ElectricityMaps
  location: "${ZONE}"
electricityMap:
  apiToken: "${TOKEN}"
  apiURL: https://api.electricitymaps.com/v4/
VALUES
umask 022
run helm upgrade --install grid-intensity-exporter \
  "$WORKDIR/grid-intensity-go/helm/grid-intensity-exporter" \
  --namespace grid-intensity --create-namespace -f "$WORKDIR/grid-intensity-values.yaml"
cat > "$WORKDIR/grid-intensity-servicemonitor.yaml" <<SM
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: grid-intensity-service-monitor
  namespace: grid-intensity
  labels:
    app.kubernetes.io/name: grid-intensity-exporter
    release: ${PROM_RELEASE}
spec:
  endpoints:
    - path: /metrics
      port: metrics
      scheme: http
      interval: 5m
      scrapeTimeout: 2m
  namespaceSelector:
    matchNames: [grid-intensity]
  selector:
    matchLabels:
      app.kubernetes.io/name: grid-intensity-exporter
SM
run kubectl apply -f "$WORKDIR/grid-intensity-servicemonitor.yaml"

# 5. Boavizta exporter. It reads EKS node types, so only install it on AWS.
if [ "$PROVIDER" = "aws" ]; then
  log "Installing the KEIT Boavizta exporter (EKS)"
  run helm upgrade --install keit-boavizta-exporter "$REPO_ROOT/helm" \
    --namespace keit --create-namespace \
    --set servicemonitor.release="$PROM_RELEASE"
else
  log "Skipping the Boavizta exporter: it supports EKS only. Embodied-emission panels stay empty."
fi

# 6. Dashboard, as a ConfigMap that the Grafana sidecar of kube-prometheus-stack imports.
log "Creating the KEIT dashboard with PUE ${PUE}"
DASHBOARD="$WORKDIR/KEIT_grafana_dashboard.json"
sed '/"name": "PUE_avr_aws"/,/"query"/ s/"query": "[^"]*"/"query": "'"$PUE"'"/' \
  "$REPO_ROOT/grafana/KEIT_grafana_dashboard.json" > "$DASHBOARD"
grep -q "\"query\": \"${PUE}\"" "$DASHBOARD" || die "could not set the PUE in the dashboard"
kubectl create configmap keit-dashboard --namespace "$PROM_NAMESPACE" \
  --from-file=KEIT_grafana_dashboard.json="$DASHBOARD" --dry-run=client -o yaml \
  | kubectl label --local -f - grafana_dashboard=1 -o yaml \
  | run kubectl apply -f -

cat <<DONE

KEIT is installed. Open Grafana:

  kubectl port-forward svc/${PROM_RELEASE}-grafana 3000:80 -n ${PROM_NAMESPACE}

then go to http://127.0.0.1:3000 (kube-prometheus-stack default login: admin / prom-operator)
and open the dashboard "Carbon intensity EKS - KEIT (Ierland)" (the title predates this script). Kepler and the carbon-intensity exporter need a few
minutes to produce data. The Electricity Maps free tier updates hourly.
DONE
