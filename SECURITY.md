# Security policy

## Reporting a vulnerability

Please do not report security problems in public issues, discussions or pull requests.

Use GitHub's private vulnerability reporting: go to the **Security** tab of this repository and choose **Report a vulnerability**. If that option is not available, email [GAP: contact address].

Please include:

- what you found and which component it affects (`source/`, `scaleway-footprint-exporter/`, a Helm chart, a dashboard);
- the version or commit you tested;
- steps to reproduce, or a proof of concept;
- what you think the impact is.

We will acknowledge your report and keep you informed as we look into it. [GAP: response-time commitment, to be agreed by the maintainers before this file is merged.]

## Supported versions

Security fixes go into the latest release of KEIT and of the Scaleway footprint exporter, published at `ghcr.io/aknostic/keit` and `ghcr.io/aknostic/scaleway-footprint-exporter`. We do not backport fixes to older releases.

## Scope

In scope: code, container images, Helm charts and manifests in this repository.

Out of scope: vulnerabilities in the projects KEIT builds on (Kepler, grid-intensity-go, Prometheus, Grafana, Metrics Server, the Boavizta API). Please report those to their own maintainers. If KEIT's configuration makes such an issue worse, tell us.

Credentials such as the ElectricityMaps token or Scaleway keys are supplied by you at install time. Never include real credentials in a report.
