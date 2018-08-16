# Machinehead

A docker-compose application manager that deploys and maintains a set of compose
projects and provides secret management for them via
[Vault](https://www.vaultproject.io/).

Machinehead is designed for single-server hobbyists who want to make use of
modern container automation tools but can't since most of them focus on cluster
technology such as Swarm and Kubernetes. Managing secrets on single-server
deployments doesn't have any of these services available for easy integration
with the compose tool so this application aims to both solve that problem and
also provide a simple way to keep compose deployments updated.

## Architecture

Machinehead is given one or more Git repositories that contain
`docker-compose.yml`files ("Projects") to watch. It will periodically fetch
updates from each one, if there is a change it will execute `docker-compose up`
for it.

In addition to that, it will export a set of secrets read from Hashicorp Vault
as environment variables for each project. This separates deployment manifests
from credentials, keeps deployment stateless and lets you update compose
projects with a simple `git push`.

## Usage

The application is currently under development so things might change.

Since this is an application intended to be spun up on a server and left alone,
configuration is done via environment variables:

- `MACHINEHEAD_TARGETS` a comma-separated list of git URLs
- `MACHINEHEAD_CHECK_INTERVAL` how frequently to check git repos for changes
- `MACHINEHEAD_CACHE_DIRECTORY` where to clone git repos
- `MACHINEHEAD_VAULT_ADDRESS` the Vault server address
- `MACHINEHEAD_VAULT_TOKEN` a vault token that has access to secrets

## Vault

Inside vault, Machinehead uses the project name, the base component of the git
repo path, as a Vault path and attempts to read values from there as
string-to-string values. It will export these values as environment variables
for each project deployment.
