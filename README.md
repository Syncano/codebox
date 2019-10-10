# Codebox (Script Runner)

[![CircleCI](https://circleci.com/gh/Syncano/codebox.svg?style=svg)](https://circleci.com/gh/codebox)

## Dependencies

- Golang version 1.13.
- docker 1.13+ and docker-compose (`pip install docker-compose`).

## Testing

- Run `make test` to run code checks and all tests with coverage. This will require Go installed on host.
- During development it is very useful to run dashboard for tests through `goconvey`. Install and run through `make goconvey`.
- Whole project sources are meant to be put in $GOPATH/src path. This is especially important during development.
- To run tests in container run: `make test-in-docker`.

## Starting locally

- Build executable binary by `make build build-wrapper` or `make build-in-docker`. They both do the same but the first one requires dependencies to installed on local machine. Second command will automatically fetch all dependencies in docker container.
- Rebuild the image by `make docker`.
- Run `make start` to spin up 1 load balancer and 1 worker instance.

## Deployment

- You need to first build a static version and a docker image. See first two steps of **Starting locally** section.
- Make sure you have a working `kubectl` installed and configured. During deployment you may also require `gpg` (gnupg) and `jinja2-cli` (`pip install jinja2-cli[yaml]`).
- Run `make deploy-staging` to deploy on staging or `make deploy-production` to deploy on production.
