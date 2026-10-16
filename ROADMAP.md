# KEIT roadmap

This is what we plan to work on, in rough order of how likely it is to happen soon. It is a statement of intent, not a promise: there are no dates, and priorities change with what adopters tell us. To influence it, open an issue or [a method proposal](https://github.com/aknostic/keit/issues/new/choose), or send a pull request.

## In progress

### Azure (AKS)

Support for Azure is being worked on. The embodied-emissions exporter currently looks up instance types on AWS EKS only; on Azure you enter your hardware by hand in [Datavizta](https://datavizta.boavizta.org/serversimpact), as described in the [README](README.md#get-embodied-emissions).

- Target: the same dynamic embodied-emissions lookup that exists for EKS, so node types are picked up as the cluster scales.
- Status: work in progress, nothing released. [GAP: current state of the Azure branch or prototype, and who is working on it]

## Next

### Google Cloud (GKE)

Planned, not started. Needs the same node-type lookup as Azure and a check of what Kepler can report on GKE VMs (see the Kepler item below). Reports from anyone running KEIT on GKE are welcome through the "I'm running KEIT" issue template.

### Dynamic grid zone

Today you set the ElectricityMaps zone once, in `grid-intensity-exporter/values.yml`. It is not detected from where your nodes run. We want to pick the zone from node topology labels (region and zone) so that multi-region clusters are measured per region. [GAP: the design is not written yet; a design note will be linked here.]

### Kepler v0.10 and later

Since Kepler v0.10 only reports energy measured through RAPL, which cloud VMs do not expose. KEIT therefore pins Kepler chart 0.6.1 (Kepler 0.8.0) on managed Kubernetes. The route off that pin depends on upstream: VM power models are being worked on in [kepler#2589](https://github.com/sustainable-computing-io/kepler/issues/2589). We plan to follow that work, contribute to it where we can, and move KEIT to current Kepler once it estimates power on VMs. Bare-metal clusters with RAPL access are not affected.

### grid-intensity-go dependency

KEIT gets ElectricityMaps data through the [grid-intensity-go](https://github.com/thegreenwebfoundation/grid-intensity-go) Prometheus exporter. It has had no release since June 2024. We are asking the maintainers about its future. If it stays unmaintained, we will vendor it or replace it with a small exporter of our own. [GAP: outcome of the conversation with the Green Web Foundation, once there is one.]

## Later

- **More provider exporters.** The [Scaleway footprint exporter](scaleway-footprint-exporter/README.md) feeds provider-reported footprint data into the dashboards. We would like the same for other providers that publish such data. [GAP: which providers publish a usable API today; to be checked before committing to any.]
- **Dashboard improvements.** Clearer colours for the SCI components ([#24](https://github.com/aknostic/keit/issues/24)), a fix for "No data" on longer time ranges ([#15](https://github.com/aknostic/keit/issues/15)), and publishing the dashboard on grafana.com ([#16](https://github.com/aknostic/keit/issues/16)).

## Non-goals

- **Audited or regulatory reporting.** KEIT gives estimates. Energy is modelled on VMs, grid intensity changes by the hour, and embodied emissions come from hardware databases with wide ranges. Do not use its output as an audited figure for CSRD or similar reporting.
- **A hosted service.** KEIT runs in your own cluster and sends nothing to us. We do not plan a SaaS version.

## Suggest a change

Open an issue, or comment on the one that covers the item. If you run KEIT, adding yourself to [ADOPTERS.md](ADOPTERS.md) also tells us what to prioritise.
