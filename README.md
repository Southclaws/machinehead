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
