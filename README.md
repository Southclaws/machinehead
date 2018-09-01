# Machinehead

_[GitOps][gitops] for single-server deployments!_

A docker-compose application manager that deploys and maintains a set of compose
projects and provides secret management for them via
[Vault](https://www.vaultproject.io/).

Machinehead is designed for single-server hobbyists who want to make use of
containers and modern [GitOps][gitops] practices but can't since most of the
tools (such as [kube-applier][kube-applier]) focus on cluster technology such as
Swarm and Kubernetes.

In addition to this lack of tooling, managing sensitive secrets such as database
credentials on single-server deployments doesn't have many solutions that
integrate with Docker nicely.

And so, Machinehead was born to solve both these problems!

## Architecture

Machinehead is essentially a background process that is given one or more Git
repositories that contain `docker-compose.yml` files. It will periodically
attempt to pull from each reository and, if there is a change it will execute
`docker-compose up` for it.

This lets you update the configuration of your containerised applications simply
by doing a `git push`!

Tip: pairs really nicely with [Watchtower][watchtower]!

<!-- In addition to that, it will export a set of secrets read from Hashicorp Vault
as environment variables for each project. This separates deployment manifests
from credentials, keeps deployment stateless and lets you update compose
projects with a simple `git push`. TODO! -->

## Usage

Machinehead is current-working-directory ("CWD") sensitive rather than
binary-path sensitive, this means you can install it with `go get` and run it
from any directory.

It doesn't currently have any official daemonising methods so it's up to you to
write your own systemd/upstart/whatever configs. You could also just use tmux or
screen and detach from the session.

### Configuration

When Machinehead is started, it will check the CWD for `machinehead.json` which
looks like:

```json
{
  "targets": [
    "git@domain:username/my-project1",
    "git@domain:username/my-project2"
  ],
  "check_interval": "10s",
  "cache_directory": "./machinehead_cache"
}
```

For best results, the directory that contains `machinehead.json` should also be
a git repository, if it is, Machinehead will also watch that repository for
changes and, if there are any, it will pull them and automatically update its
own configuration if there are changes to `machinehead.json`.

### Global Environment Variables

Machinehead will also search the CWD for a file named `.env`, if it finds one,
it will attempt to read it as `key=value` format and pass the fields to all
instances of `docker-compose` which means you can set globally shared variables
for all your projects.

### Vault

Vault isn't currently supported but it's planned. The idea is, you'll be able to
create per-project secrets inside Vault that contain sensitive configuration
variables such as database credentials etc.

Using the example targets above, Machinehead will read `kv` values from
`/secrets/my-project1` for `my-project1` and export them as environment
variables for the `docker-compose` call.

[gitops]: https://www.weave.works/blog/gitops-operations-by-pull-request
[kube-applier]: https://github.com/box/kube-applier
[watchtower]: https://github.com/v2tec/watchtower
