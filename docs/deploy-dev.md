# Dev Deployment

The GitHub Actions workflow in `.github/workflows/ci-cd.yml` runs checks for pull requests and pushes. A successful push to `master` is deployed to the configured dev server over SSH.

## Dev server prerequisites

The server must have:

- Git access to `git@github.com:faraquic/lotty-ab-platform.git`.
- Go 1.27, Podman or Docker Compose, and the project tools required by `make up`.
- A checked-out copy of the repository.
- A valid `config.dev.json` in the repository.
- Ports `8080`, `8081`, `8082`, `8083`, `5433`, `6379`, `9000`, and `9001` available as required by Compose.

The deployment command updates the checkout to `origin/master` and runs `CONFIG_FILE=config.dev.json CONFIG_NAME=config.dev.json make up`. This starts the infrastructure, waits for PostgreSQL, applies migrations, and rebuilds the application containers with `environment: "dev"`.

## GitHub environment

Create a GitHub environment named `dev` and add these secrets:

- `DEV_HOST`: DNS name or IP address of the server.
- `DEV_USER`: SSH user.
- `DEV_SSH_PORT`: SSH port, for example `22`.
- `DEV_SSH_KEY`: private Ed25519 key allowed to log in as `DEV_USER`.
- `DEV_KNOWN_HOSTS`: the pinned output of `ssh-keyscan -H <host>`; do not leave this empty.
- `DEV_APP_DIR`: absolute path to the repository checkout, for example `/srv/lotty-ab-platform`.

The deploy job runs only for a successful push to `master`. Pull requests run tests and image builds but never receive deployment credentials.

## First-time server setup

```bash
git clone git@github.com:faraquic/lotty-ab-platform.git /srv/lotty-ab-platform
cd /srv/lotty-ab-platform
make dev-up
CONFIG_FILE=config.dev.json CONFIG_NAME=config.dev.json make up
```

Verify that the host can reach the application gateway:

```bash
curl --fail http://127.0.0.1:8080/health
curl --fail http://127.0.0.1:8080/api/v1/panel/health
```

Keep server-specific credentials out of workflow logs and do not store private SSH keys in the repository. The committed dev file currently contains development-only placeholder credentials; replace them with deployment secrets before exposing the server.
