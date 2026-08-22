# wslc-compose

A small `docker compose`-style CLI for [WSL containers](https://learn.microsoft.com/en-us/windows/wsl/wsl-container) (`wslc.exe`). `wslc` doesn't understand compose files yet, so this reads one and drives `wslc network`/`wslc volume`/`wslc build`/`wslc run` to bring services up the way `docker compose` would. It's a thin wrapper, not a reimplementation — every actual container/network/volume operation goes through the real `wslc` binary.

## Requirements

- Windows with the WSL container feature installed (`wslc version` should work in a terminal).
- No native Go install needed — building and testing both happen through a containerized Go toolchain via `wslc` itself (see below).

## Build

```powershell
.\build.ps1
```

This runs a `golang` image through `wslc run`, cross-compiles for `windows/amd64`, and drops `wslc-compose.exe` in the repo root.

## Usage

```powershell
wslc-compose <up|down|start|stop> [-f compose-file] [-p project-name]
```

- **`up`** — creates networks and volumes, builds any service with a `build:` key, then starts services in `depends_on` order. If a container/network/volume already exists and was created by this project, it's reused; if it exists and belongs to something else, `up` refuses to touch it.
- **`down`** — stops and removes the project's containers and networks (in reverse dependency order). **Volumes are left alone**, same as `docker compose down` without `-v`.
- **`start`** / **`stop`** — start or stop existing containers only; they never create or remove anything.

If `-f` is omitted, it looks for `compose.yaml`, `compose.yml`, `docker-compose.yaml`, or `docker-compose.yml` in the current directory. The project name comes from `-p`, then a `name:` key in the compose file, then the compose file's directory name — same precedence as `docker compose`.

Example:

```yaml
name: myapp

services:
  db:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD: example
    volumes:
      - data:/var/lib/postgresql/data
    networks:
      backend:
        aliases: [database]

  app:
    build: .
    depends_on: [db]
    ports:
      - "8080:80"
    networks: [backend]

networks:
  backend:
    driver: bridge

volumes:
  data:
```

Services on the same network resolve each other by service name (and any declared aliases) through `wslc`'s built-in DNS, exactly like Compose.

### Supported compose-spec subset

`image`, `build` (context/dockerfile/args/target), `container_name`, `command`, `entrypoint`, `environment`, `env_file`, `ports` (short and long syntax), `volumes` (short and long syntax, named or bind), `networks` (list or map with aliases), `depends_on` (list or map), `labels`, `hostname`, `domainname`, `user`, `working_dir`, `dns`, `dns_search`, `shm_size`, `mem_limit`, `cpus`, `tmpfs`, `stop_signal`, `ulimits`; top-level `networks:` and `volumes:` (`driver`, `driver_opts`, `labels`, `name`, `external`).

Keys `wslc` has no equivalent for yet — `restart`, `healthcheck`, `cap_add`/`cap_drop`, `sysctls`, `deploy.*`, `read_only`, `profiles`, `configs`, `secrets`, and anything else not listed above — are reported as a warning and otherwise ignored rather than failing the whole file. A `depends_on` health condition is downgraded to plain start-order with a warning, since `wslc` has no healthcheck primitive to gate on.

### How it tracks its own resources

There's no local state file. Every network/volume/container `up` creates is labeled with `com.wslc-compose.project`/`.service`/`.network`/`.volume`, and resource names are deterministic (`<project>-<service>`, etc.), so `down`/`start`/`stop` just recompute the expected names from the same compose file and verify the label before touching anything — the same approach `docker compose` itself uses.

## Development

A [devcontainer](.devcontainer/devcontainer.json) is provided for editing — it's Linux-based, so `wslc.exe` isn't available inside it. That's fine: everything except the real end-to-end path is tested against a fake `wslc` runner (`internal/wslc/wslcfake`), which behaves identically on Linux and Windows.

```powershell
.\test.ps1
```

runs both halves:

1. Unit tests (`go test ./...`) inside a Linux container, against the fake runner.
2. A Windows-only integration test (`integration/`, `//go:build windows`) that's cross-compiled in the same container and then executed natively, exercising the real `wslc.exe`. It skips itself if `wslc` isn't on `PATH`.

## A note on how this was built

This was put together in a "vibe coding" session with Claude Code — most of the design (compose-spec scope, state tracking, naming, the label-based ownership model) came out of an interactive back-and-forth, and the code was generated and iterated on by the assistant rather than typed by hand. The `wslc` CLI surface referenced here (flags, subcommands, JSON output shapes) was verified directly against an installed `wslc.exe` rather than assumed from docs, and the up/down/start/stop lifecycle was manually smoke-tested end to end (including cross-container DNS resolution) before being wrapped in the automated test suite.

That said: `wslc`/WSL containers is a Microsoft feature still in public preview, its CLI surface can and likely will change, and this project hasn't seen production use. Read the code before trusting it with anything you care about, especially around the parts that mutate state (`up`/`down`).
