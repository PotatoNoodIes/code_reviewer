# AI Code Reviewer

A GitHub Action that reviews pull requests like a senior engineer — bugs,
security issues, simplification opportunities, and style — using a
self-hosted [Ollama](https://ollama.com) model instead of a paid external
API. Written in Go.

For each changed file, it fetches the full file content at the PR's head
commit (not just the diff) so the model has real context, then posts
findings as inline GitHub review comments on the exact lines they apply
to.

## How it works

1. Triggered by a `pull_request` event, it reads the event payload to find
   the PR's owner/repo/number/head SHA.
2. Lists changed files, applies `ignore-patterns` and the `max-files` cap
   (largest diffs reviewed first).
3. For each remaining file: parses the unified diff to find which lines a
   comment may legally target, fetches the full file content via the Git
   Blob API, and sends both to Ollama with a system prompt that fixes a
   senior-dev persona and four categories (`bug`, `security`,
   `simplification`, `style`).
4. Ollama's output is constrained to a JSON schema; malformed output is
   retried, and a file that never produces valid JSON is skipped (reported
   in the job summary) rather than failing the whole run.
5. Every finding is re-validated against the file's real commentable
   lines before posting — the model's own claims are never trusted.
6. Comments are posted one GitHub review per file (not one review for the
   whole PR), so a bad line-mapping in one file can't take down every
   other file's comments. A content hash embedded in each comment body
   prevents duplicate posts on repeated `synchronize` pushes.

## Usage

```yaml
name: AI Code Review
on:
  pull_request:
    types: [opened, synchronize]

permissions:
  pull-requests: write   # required — the default GITHUB_TOKEN is read-only otherwise

jobs:
  review:
    runs-on: self-hosted   # see "Ollama reachability" below
    steps:
      - uses: <owner>/ai-code-reviewer@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          ollama-url: http://localhost:11434
          ollama-model: qwen2.5-coder:7b
```

### Inputs

| Input | Default | Description |
|---|---|---|
| `github-token` | *(required)* | Token used to read PR data and post comments. |
| `ollama-url` | `http://localhost:11434` | Base URL of the Ollama server. See reachability note below. |
| `ollama-model` | `qwen2.5-coder:7b` | Model to use. A code-focused model works far better than a general chat model. |
| `ollama-format-mode` | `schema` | `schema` (JSON-Schema constrained decoding, needs Ollama 0.5+), `json` (older Ollama's plain JSON mode), or `none`. |
| `max-files` | `15` | Cap on files reviewed per PR; the rest are skipped and reported in the job summary. |
| `ignore-patterns` | vendor/node_modules/lockfiles/build output | Comma-separated globs of paths to skip. |
| `review-event` | `COMMENT` | `COMMENT`, `APPROVE`, or `REQUEST_CHANGES`. |
| `dry-run` | `false` | Print findings to the log instead of posting them. |
| `ollama-concurrency`, `max-file-lines` | `1`, `1200` | Reserved for future use; not yet enforced (see Limitations). |

## Ollama reachability — read this before deploying

This action assumes Ollama runs on hardware your GitHub Actions runner can
reach — realistic for a self-hosted runner, not for GitHub-hosted runners.

**Important nuance beyond that:** this ships as a *Docker container
action*, which runs in its own network namespace, isolated from the
runner host. `ollama-url: http://localhost:11434` will resolve to
*inside the container*, not the host running Ollama — even on a
self-hosted runner. Two ways to fix it:

- Bind Ollama to all interfaces (`OLLAMA_HOST=0.0.0.0` on the Ollama
  server) and set `ollama-url` to the runner host's real LAN IP, or to
  the Docker bridge gateway IP reachable from inside the container
  (commonly `172.17.0.1` on Linux, but confirm with `ip route` inside a
  test container).
- If you run the self-hosted runner itself inside Docker, start it with
  `--network host` so job containers inherit host networking and
  `localhost` resolves as expected.

## Local development

```sh
make build test vet   # unit tests, no network required
make integration-test # hits the real public GitHub API (read-only, unauthenticated)
```

To try it against a real PR without posting anything:

```sh
GITHUB_TOKEN=<token> DRY_RUN=true go run ./cmd/reviewer --owner <owner> --repo <repo> --pr <number>
```

To build and smoke-test the container:

```sh
make docker-build
docker run --rm \
  -e GITHUB_EVENT_PATH=/testdata/event.json -e GITHUB_EVENT_NAME=pull_request \
  -e GITHUB_TOKEN=<token> -e DRY_RUN=true \
  -v "$PWD/testdata/event.json:/testdata/event.json:ro" \
  ai-code-reviewer:local
```

## Known v1 limitations

- No stale-comment resolution: the REST API has no "resolve thread"
  endpoint (that needs GraphQL's `resolveReviewThread`), so comments for
  since-fixed issues stay open. Deferred to v2.
- `max-file-lines` and `ollama-concurrency` are accepted as inputs but not
  yet enforced — files are reviewed sequentially and oversized files
  aren't capped or skipped by line count.
- Renamed-with-no-content-change files, and diffs GitHub declines to
  compute for being too large, are skipped (no diff to validate findings
  against), not reviewed.
- JSON-Schema-constrained decoding (`ollama-format-mode: schema`) needs a
  reasonably recent Ollama; fall back to `json` mode on older servers.
