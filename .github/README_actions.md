GitHub Actions in this repository.

## Build and Publish KEIT (`workflows/build-and-docker.yml`)

- On every pull request that changes `source/`, the KEIT image is built as a check. Nothing is pushed, so this is safe for pull requests from forks.
- A new image is only published when a maintainer pushes a `keit-v<version>` tag.
- Creating, changing or deleting `keit-v*` tags is limited to repository admins (maintainers) by the tag ruleset "Protect KEIT release tags".
- The tag must point to a commit on `main`, otherwise the release stops.

Process of releasing a new docker image (admins only):

1. Merge the changes to `main` through a pull request.
2. Tag the commit on `main` and push the tag:
   ```bash
   git switch main
   git pull
   git tag keit-v0.0.12
   git push origin keit-v0.0.12
   ```
3. The workflow pushes `ghcr.io/aknostic/keit:<version>` and moves `ghcr.io/aknostic/keit:latest` to it.

## Build and Publish scaleway-footprint-exporter (`workflows/build-scaleway-footprint-exporter.yml`)

Works the same as KEIT: build-only on pull requests, released when a maintainer pushes a `scaleway-footprint-exporter-v<version>` tag on `main` (restricted to repository admins by the tag ruleset "Protect exporter release tags"). The tag must match `appVersion` in the chart, and there is no `latest` tag. See [Releasing](../scaleway-footprint-exporter/README.md#releasing-publishing-a-new-image) in the exporter's README.

## Dependabot (`dependabot.yml`)

Opens weekly pull requests for Go modules (`source/`, `scaleway-footprint-exporter/`), the Dockerfile base images and the GitHub Actions used in the workflows.
