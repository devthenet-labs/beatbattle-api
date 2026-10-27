# beatbattle-api

The Beat Battle api, served under `/api` on port 8080. People save 16-step drum patterns, vote on them, read the
leaderboard and ask for WAV renders. It is scaffolded by `maul init app` so that
[maul](https://github.com/devthenet-labs/maul) can build intents in it and preview its pull requests. Standard library
only; it writes nothing to disk. The contract is shared with the other two repositories (`beatbattle-web` serves the UI
at `/`, `beatbattle-render` synthesises WAV at `/render`); only the browser composes them, same-origin.

## HTTP API

Errors are JSON `{"error": "<message>"}`. Request bodies are limited to 16 KiB (413 beyond that). No auth yet.

| Method and path                      | Result                                                                                   |
| ------------------------------------ | ---------------------------------------------------------------------------------------- |
| `GET /api/healthz`                   | `{"status":"ok","revision":"<sha>"}`                                                     |
| `POST /api/patterns`                 | 201 with the stored pattern (`id` is `p_` + 6 hex); invalid input 400; past 1000 patterns 507 `store full` |
| `GET /api/patterns?limit=N`          | bare JSON array, newest first; default 20, maximum 100 (larger is clamped; non-numeric or < 1 is 400) |
| `GET /api/patterns/{id}`             | the pattern, or 404                                                                      |
| `POST /api/patterns/{id}/votes`      | 200 `{"votes": N}`, or 404                                                               |
| `GET /api/leaderboard`               | bare JSON array: top 10 by votes, ties to the earlier `createdAt`                        |
| `POST /api/patterns/{id}/renders`    | 202 `{"renderId":"r_…","patternId":"…","status":"pending"}`, or 404                      |
| `GET /api/renders/{id}`              | that status (always `pending` in v0), or 404                                             |

A pattern is `{id, name, author, bpm, swing, tracks:{kick,snare,hat}, votes, createdAt}`; each track is exactly 16
booleans. `name` is 1-40 characters after trimming with no control characters, `author` 1-24 of `[A-Za-z0-9_-]`, `bpm` an
integer 60-200, `swing` an integer 0-60. Unknown fields and tracks are rejected; `id`, `votes` and `createdAt` are
ignored on input.

## Layout

- `internal/battle`: the pure core (validation, `Store` and `RenderQueue` ports, `Service`, `Rank`). No `net/http`;
  the clock and random source are injected.
- `internal/memstore`: the in-memory adapters (a mutex and maps). A database and a real queue replace them later.
- `internal/httpapi`: the thin HTTP adapter. `server.go` wires it with the real clock and `crypto/rand`.

## Develop

Use Go 1.26.6. The service listens on `:8080`; set `ADDR` (for example `ADDR=127.0.0.1:9090`) to change it.

```sh
gofmt -l .
go vet ./...
go test -race ./...
go run .
curl -s localhost:8080/api/healthz
```

The root `Dockerfile` builds the runtime image a preview runs: a static binary on distroless, uid 65532, no shell,
listening on 8080. A preview runs it with a read-only root filesystem and no writable `/tmp`:

```sh
docker build -t bb-api:local .
docker run --rm --read-only --cap-drop ALL --security-opt no-new-privileges \
  -p 127.0.0.1:8080:8080 bb-api:local
curl -s localhost:8080/api/healthz
```

`.maul/Dockerfile` is a different image: the one maul's coding agent works in, which is maul's agent base plus the
pinned Go toolchain and this module's dependencies, offline. `.maul/agent.yaml` declares its immutable tag.

## CI and images

- `test` (`ci.yml`) tests the exact head and builds the runtime image without credentials.
- `agent image` (`agent-image.yml`) builds the agent image from `main`, also without credentials.
- `publish images` (`publish-images.yml`) hands each successful build to its trusted publisher, which pushes it to ECR.
  [`.github/actions/publish/README.md`](.github/actions/publish/README.md) describes the trust boundary.

### Repository variables

None of them is secret. Set them on this repository, never on the organization: an organization variable reaches every
repository with these workflows, a template repository and its copies included. maul's reference terraform module
(`deploy/terraform/aws/modules/app`) outputs the configuration as `github_variables_dotenv`; export it from your
terraform root and set it in one command:

```sh
terraform output -raw maul_app_beatbattle_api_variables | gh variable set -f - --repo devthenet-labs/beatbattle-api
```

Set the configuration first and the two `*_PUBLISH_ENABLED` gates last:

| Variable                   | Value                                                       | What it is                                                          |
| -------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------- |
| `PUBLISH_REPOSITORY_ID`    | `gh api repos/devthenet-labs/beatbattle-api --jq .id`       | this repository's immutable numeric ID, which the guard checks      |
| `PUBLISH_OWNER_ID`         | `gh api repos/devthenet-labs/beatbattle-api --jq .owner.id` | its owner's immutable numeric ID                                    |
| `AWS_REGION`               | `us-east-1`                                                 | the registry's AWS region                                           |
| `ECR_REGISTRY`             | `377946145366.dkr.ecr.us-east-1.amazonaws.com`              | the ECR registry host                                               |
| `AGENT_IMAGE_REPOSITORY`   | `maul/app-envs/beatbattle-api`                              | the agent image's ECR repository, which `.maul/agent.yaml` names    |
| `AGENT_ROLE_ARN`           | the agent publisher role ARN                                | assumed only by `publish-agent.yml`                                 |
| `AGENT_SIGNING_KEY`        | the signing key's `awskms:///` URI, or unset                | optional: the KMS key the agent image is signed with                |
| `RUNTIME_IMAGE_REPOSITORY` | `maul/previews/beatbattle-api`                              | the runtime (preview) image's ECR repository                        |
| `RUNTIME_ROLE_ARN`         | the runtime publisher role ARN                              | assumed only by `publish-runtime.yml`                               |
| `AGENT_PUBLISH_ENABLED`    | `true`                                                      | publishes the agent image; anything else skips it                   |
| `PREVIEW_PUBLISH_ENABLED`  | `true`                                                      | publishes runtime images; set it last, once previews are configured |

### Trusted workflows

Only these two workflows assume an AWS role, and each role trusts only its own workflow at `main`:

| Trusted workflow (`job_workflow_ref`)                                                 | Assumes            | Publishes                                                                            |
| ------------------------------------------------------------------------------------- | ------------------ | ------------------------------------------------------------------------------------ |
| `devthenet-labs/beatbattle-api/.github/workflows/publish-runtime.yml@refs/heads/main` | `RUNTIME_ROLE_ARN` | the runtime image as `sha-<commit>`, and `main-<commit>` for a default-branch commit |
| `devthenet-labs/beatbattle-api/.github/workflows/publish-agent.yml@refs/heads/main`   | `AGENT_ROLE_ARN`   | the agent image as the `toolchain-v<N>` tag `.maul/agent.yaml` declares              |

### Changing the agent toolchain

Edit `.maul/Dockerfile` (or the dependencies in `go.mod`) and bump the `toolchain-v<N>` tag in `.maul/agent.yaml` in
the same commit: published tags are immutable, so a changed image needs a new tag.
