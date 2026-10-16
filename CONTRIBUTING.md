# Contributing to KEIT

Thank you for taking the time to help. KEIT is an Apache 2.0 licensed estimate of Kubernetes emissions, and it gets better when people test it on clusters we do not run ourselves.

By taking part you agree to follow our [Code of Conduct](CODE_OF_CONDUCT.md). To report a security problem, read [SECURITY.md](SECURITY.md) first and do not open a public issue.

## Contributing without code

Some of the most useful contributions are not code:

- **Method improvements.** If you think the way KEIT combines energy, grid carbon intensity and embodied emissions is wrong or too coarse, open a feature or method proposal. Say what you would change and why, with a source if you have one.
- **Emission factors and PUE sources.** A published PUE or emission factor for a provider or region, with a link to where the provider states it, helps everyone who runs there. The README lists the PUE values we know about.
- **Testing on your provider.** KEIT is tested on bare metal and adapted to AWS EKS. If you run it on another provider or Kubernetes distribution, tell us what worked and what did not, using the "I'm running KEIT" issue template.
- **Bug reports and documentation fixes.** Unclear installation steps are bugs too.

## What is in the repository

| Directory | What it is |
|---|---|
| `source/` | The KEIT Go program and its `Dockerfile`. |
| `scaleway-footprint-exporter/` | A Prometheus exporter for Scaleway's footprint data, with its own Helm chart. |
| `grid-intensity-exporter/` | Values and manifests for the grid-intensity-go exporter. |
| `helm/` | Helm chart for KEIT. |
| `grafana/` | The KEIT dashboard and the reusable dashboards in `grafana/dashboards/`. |

## Development setup

You need Go (the version in `source/go.mod`), Docker, and for end-to-end checks a Kubernetes cluster with Prometheus and Grafana. The [README](README.md) shows how to install Kepler, the grid-intensity exporter, Prometheus and Grafana with Helm.

Fork the repository, clone your fork and create a branch from `main`:

```bash
git clone git@github.com:<your-user>/keit.git
cd keit
git switch -c my-change
```

## Building the exporters

KEIT (`source/`):

```bash
cd source
go build ./...
docker build -t keit .
```

Scaleway footprint exporter:

```bash
cd scaleway-footprint-exporter
go run .                      # needs SCW_ORG_ID and SCW_SECRET_KEY
docker build -t scaleway-footprint-exporter .
```

Pull requests that change `source/` or `scaleway-footprint-exporter/` get a build-only CI check. Nothing is published from a pull request. Releases are made by maintainers pushing a version tag, as described in [`.github/README_actions.md`](.github/README_actions.md); you do not need to tag or push images.

## Testing dashboard changes

1. In Grafana go to Dashboards, then Import, and upload the changed `.json` file from `grafana/`.
2. Choose your Prometheus datasource when asked. For the dashboards in `grafana/dashboards/`, set the **Cluster** variable to match your `kubernetes_cluster` external label, or use `.+` to match all.
3. Check that every panel shows data rather than "No data". A panel that is empty on your cluster because a metric is missing is useful information; say so in the pull request.
4. Make sure the metrics the dashboard needs are listed. [`grafana/dashboards/README.md`](grafana/dashboards/README.md) documents the required metrics for each dashboard (for example Kepler, grid-intensity-go, kube-state-metrics and cadvisor). If you add a dependency on a new metric, add it there.
5. When you export a dashboard from Grafana, keep the diff small: avoid committing changes to unrelated panels, datasource UIDs or the dashboard `id`. Do not leave your own PUE, cluster name or project identifiers in the JSON unless they are the documented defaults.
6. Include a screenshot of the changed panels in the pull request.

## Commit messages

Match the existing history: a short, capitalised, imperative summary line with no trailing full stop, and the pull request number added when the change is merged. For example:

```
Release KEIT by keit-v* tag; build-only on pull requests (#88)
Move CI to Ubuntu 26.04 and fix KEIT image build (#85)
```

Keep one logical change per commit. Explain the why in the body when it is not obvious from the diff.

## Pull requests

- Open an issue first for anything larger than a small fix, so we can agree on the approach before you spend time on it.
- Keep pull requests focused. Fill in the pull request template.
- Update the README or the relevant sub-README when behaviour or setup changes.
- A maintainer will review when they can. This is a small project and replies may take a few days.

## Licence

By contributing you agree that your contribution is licensed under the [Apache License 2.0](LICENSE), the same licence as the rest of the project.
