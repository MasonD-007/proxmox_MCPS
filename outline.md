# Proxmox MCP Server — Architecture & Multi-Agent Issue Breakdown

## Stack decision

- **Language: Go.** Plays to your strengths and the registration trick below is a pure Go idiom (the compiler does the wiring for you).
- **MCP layer: the official `github.com/modelcontextprotocol/go-sdk/mcp`** (maintained with Google, past v1.0). It gives you `mcp.Server`, typed `AddTool` with JSON-schema generated from struct tags, plus first-class Resources and Prompts, over stdio or streamable-HTTP transports. Targets recent Go (pin `go 1.24`+).
- **Proxmox client: `github.com/luthermonson/go-proxmox`.** Token auth, context support, TLS options, and object models (`Node`, `VirtualMachine`, `Container`, `Snapshot`, `RRDData`) so you are not hand-rolling `/api2/json` calls. `github.com/Telmate/proxmox-api-go` is the fallback if you hit an unsupported endpoint.

> Note on prior art: the existing Proxmox MCP servers (mostly Python/Node) centralize tool registration in one file — e.g. an `__init__.py` that lists all 115 tools. That single file is exactly the merge-conflict magnet a worktree swarm must avoid. The design below removes it entirely.

---

## Architectural strategy for avoiding merge conflicts

### The enemy: shared mutable surfaces

In a worktree swarm, conflicts come from files that *every* feature must edit:
1. A central router/dispatcher mapping tool name → handler.
2. A registration list/array of tools.
3. The Proxmox client (everyone needs a different API call on it).
4. `main.go` wiring and `go.mod`.

We neutralize 1–3 with two Go-specific properties and contain 4 in Phase 1.

### Mechanism 1 — Compile-time self-registration (`init()` in one package)

Go compiles **every `.go` file in a package directory** automatically, and runs **every `init()`** when that package is imported. So:

- All feature files live in one package, `internal/features`.
- Each feature file registers itself in its own `init()`.
- `cmd/proxmox-mcp/main.go` blank-imports that package **once** (set up in Phase 1, then frozen).

Adding `internal/features/snapshot.go` requires **zero edits** to `main.go`, the server, or any import list — the compiler finds the file and its `init()` runs. There is no central array to append to.

### Mechanism 2 — Methods on a shared struct, declared in per-feature files

Go lets you attach methods to a type from *any* file in the type's package. The Proxmox client wrapper (`proxmox.Service`) is **defined once** in Phase 1 (`service.go`, holds the authed `go-proxmox` client + shared helpers). Each feature then adds the specific API call it needs as a method **in its own file**:

```go
// internal/proxmox/service_snapshot.go  (net-new, owned by the Snapshot agent)
func (s *Service) CreateSnapshot(ctx context.Context, node string, vmid uint64, name, descr string) (string, error) {
    vm, err := s.vm(ctx, node, vmid) // shared helper from service.go
    if err != nil { return "", err }
    task, err := vm.NewSnapshot(ctx, name) // delegate to go-proxmox
    if err != nil { return "", err }
    return s.wait(ctx, task) // shared UPID-waiting helper from service.go
}
```

No edit to `service.go`. The Snapshot agent owns `service_snapshot.go` **and** `features/snapshot.go` — a vertical slice of two net-new files.

### The feature contract

Every Phase-2 agent produces a fixed, non-overlapping set of net-new files and **touches nothing else**:

| File | Purpose |
|---|---|
| `internal/features/<feature>.go` | Builds the spec + handler, registers in `init()` |
| `internal/features/<feature>_test.go` | Unit test against a local fake |
| `internal/proxmox/service_<area>.go` | The specific Proxmox API method(s) |
| `docs/tools/<feature>.md` | Tool/resource/prompt documentation |

### The registry (the seam Phase 1 freezes)

```go
// internal/registry/registry.go  (Phase 1, frozen)
type ToolHandler func(ctx context.Context, svc *proxmox.Service, req *mcp.CallToolRequest) (*mcp.CallToolResult, error)

type ToolSpec struct {
    Tool    *mcp.Tool   // name, description, JSON input schema
    Handler ToolHandler
}

func RegisterTool(s ToolSpec)         { mu.Lock(); tools = append(tools, s); mu.Unlock() }
func RegisterResource(s ResourceSpec) { /* ... */ }
func RegisterPrompt(s PromptSpec)     { /* ... */ }

func Tools() []ToolSpec { /* snapshot copy */ }   // consumed by the server
```

Critical detail: `init()` runs *before* `main()`, so features cannot capture a live `*proxmox.Service`. They register a **handler value**; the server injects the single constructed `Service` at call time. This keeps the Service out of the feature's construction path and keeps handlers pure.

A feature file then looks like:

```go
// internal/features/snapshot.go
func init() {
    registry.RegisterTool(registry.ToolSpec{
        Tool: &mcp.Tool{Name: "proxmox_snapshot", Description: "Create/list/delete VM or LXC snapshots."},
        Handler: handleSnapshot,
    })
}
func handleSnapshot(ctx context.Context, svc *proxmox.Service, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) { /* ... */ }
```

### Containing the residual shared surfaces

- **`go.mod` / `go.sum`.** Phase 1 seeds *all* common deps (`go-sdk`, `go-proxmox`, logging) so Phase-2 features add none. If a feature truly needs a new dep, that is a rare, trivially-resolved 3-way merge; the merge-time rule is "run `go mod tidy`."
- **Package-symbol namespace.** Because `internal/features` and `internal/proxmox` are each one package spanning many files, two agents could both declare a package-level `var defaultTimeout`. **Convention: every package-level identifier is prefixed with the feature name** (`snapshotDefaultTimeout`, `lxcCreateSchema`). Handlers/types stay unexported and feature-scoped. This is the one discipline reviewers must enforce.
- **`main.go`, `server.go`, CI, README.** Owned by Phase 1, declared frozen. No Phase-2 issue lists them in its blast radius.

### Worktree workflow notes

- Branch each agent off the **post-Phase-1 commit**; never start Phase 2 until Phase 1 is merged to `main`, or the frozen-substrate guarantee breaks.
- Recommend `git worktree add ../wt-snapshot feat/snapshot` per agent; each tree is a clean checkout sharing one `.git`.
- CI gate (Phase 1): `golangci-lint`, `go vet`, `go test ./...`, `go build`. Because features are independent packages-of-files, a broken feature fails only its own tests — the swarm degrades gracefully.

### Scaling escape hatch

If the single-package-namespace convention ever feels fragile at higher feature counts, switch each feature to its own **subpackage** and generate the blank-import manifest with `go:generate` scanning `internal/features/*/`. That restores full namespace isolation while keeping the manifest a regenerated (never hand-edited) file. Not needed at this scope — the convention is enough.

---

## Phase 1 — The Skeleton (sequential)

These establish the frozen substrate and **must land in order on `main`** before any Phase-2 work begins.

### [Phase 1] Issue #1: Repo scaffolding & module bootstrap
- **Description:** Initialize the Go module, directory layout, and a runnable (empty) entrypoint. Seed all shared third-party deps so feature agents never edit `go.mod`.
- **Expected Files Touched:** `go.mod`, `go.sum`, `cmd/proxmox-mcp/main.go`, `internal/features/doc.go` (package doc only), `.gitignore`, `LICENSE`, `README.md`, `Makefile`.
- **Acceptance Criteria:** `go build ./...` succeeds; `go-sdk`, `go-proxmox`, and a structured logger are present in `go.mod`; `main.go` blank-imports `internal/features` and exits cleanly; layout matches the documented structure.

### [Phase 1] Issue #2: Registry & plugin contract
- **Description:** Implement the registration package: `ToolSpec`/`ResourceSpec`/`PromptSpec`, the `Register*` functions (mutex-guarded), and the snapshot accessors the server consumes. Define the `ToolHandler`/`ResourceHandler`/`PromptHandler` signatures that inject `*proxmox.Service` at call time.
- **Expected Files Touched:** `internal/registry/registry.go`, `internal/registry/registry_test.go`.
- **Acceptance Criteria:** A test feature can register a tool from an `init()` and have it appear in `registry.Tools()`; concurrent registration is race-free (`go test -race`); zero dependency on the `features` package (no import cycle).

### [Phase 1] Issue #3: Proxmox Service substrate
- **Description:** Build `proxmox.Service` wrapping the `go-proxmox` client. Implement: client construction from env (`PROXMOX_HOST`, `PROXMOX_TOKEN_ID`, `PROXMOX_TOKEN_SECRET`, `PROXMOX_INSECURE_TLS`), token auth, TLS config, and the **shared helpers** every feature will reuse — `vm(ctx,node,vmid)`, `container(ctx,node,vmid)`, `node(ctx,name)`, and `wait(ctx, *Task)` for UPID polling. No feature-specific API calls here.
- **Expected Files Touched:** `internal/proxmox/service.go`, `internal/proxmox/config.go`, `internal/proxmox/service_test.go`.
- **Acceptance Criteria:** `Service` connects with a token and returns version via a smoke test (mockable via `gock`); helpers resolve a guest/node by id; `wait` blocks until a task UPID completes or ctx cancels; **no method named after a feature operation exists** (substrate stays generic).

### [Phase 1] Issue #4: MCP server core & transport
- **Description:** Construct the `mcp.Server`, read every spec from the registry, adapt each handler into the SDK signature (closing over the single `*proxmox.Service`), and serve over stdio (with a flag for streamable-HTTP). This is the "core router," but it routes *from the registry* — it is never edited per feature.
- **Expected Files Touched:** `internal/server/server.go`, `internal/server/server_test.go`. (Updates the `main.go` body created in Issue #1 to call `server.Run`.)
- **Acceptance Criteria:** Server boots, advertises all registered tools/resources/prompts via `tools/list` etc.; a registered no-op tool is callable end-to-end over stdio; `--http` flag switches transport; graceful shutdown on SIGINT.

### [Phase 1] Issue #5: CI/CD pipeline
- **Description:** GitHub Actions: lint (`golangci-lint`), `go vet`, `go test -race ./...`, `go build`. Add a release workflow that builds tagged binaries. Configure branch protection expectations in the README.
- **Expected Files Touched:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.golangci.yml`.
- **Acceptance Criteria:** CI runs green on `main`; PRs are gated on lint+test+build; a tag triggers a binary build artifact.

### [Phase 1] Issue #6: Contributor guide + golden reference feature
- **Description:** The highest-leverage de-risking task for the swarm. Write `CONTRIBUTING.md` specifying the feature contract (the 3–4 net-new files, the symbol-prefix convention, the `init()` pattern). Ship one fully-worked reference feature — `proxmox_version` (read-only) — demonstrating spec + handler + `init()` registration + `service_version.go` method + unit test + doc. Agents copy this as their template.
- **Expected Files Touched:** `CONTRIBUTING.md`, `internal/features/version.go`, `internal/features/version_test.go`, `internal/proxmox/service_version.go`, `docs/tools/version.md`.
- **Acceptance Criteria:** `proxmox_version` is invocable and returns the PVE version; the guide documents exactly which files a feature may and may not touch; a reviewer checklist for the prefix convention is included.

---

## Phase 2 — The Swarm (fully parallel)

Each issue is one agent, one worktree. Blast radii are disjoint by construction — verify no two issues list the same path.

### [Phase 2] Issue #7: List Nodes tool
- **Description:** `proxmox_list_nodes` — return cluster nodes with status, uptime, CPU/mem totals.
- **Expected Files Touched:** `internal/features/nodes.go`, `internal/features/nodes_test.go`, `internal/proxmox/service_nodes.go`, `docs/tools/nodes.md`.
- **Acceptance Criteria:** Tool lists all nodes with online/offline status; handles empty cluster; unit test passes against a faked node list.

### [Phase 2] Issue #8: List Guests tool
- **Description:** `proxmox_list_guests` — enumerate QEMU VMs and LXC containers across nodes, with `vmid`, name, type, node, and run state. Optional `node`/`type` filters.
- **Expected Files Touched:** `internal/features/inventory.go`, `internal/features/inventory_test.go`, `internal/proxmox/service_inventory.go`, `docs/tools/inventory.md`.
- **Acceptance Criteria:** Returns both VMs and containers distinguished by type; filters work; stable ordering by `vmid`.

### [Phase 2] Issue #9: VM power management tool
- **Description:** `proxmox_vm_power` — start / stop / shutdown / reboot a QEMU VM. Block on the resulting task UPID via the shared `wait` helper; report final state.
- **Expected Files Touched:** `internal/features/vm_power.go`, `internal/features/vm_power_test.go`, `internal/proxmox/service_vm_power.go`, `docs/tools/vm_power.md`.
- **Acceptance Criteria:** Each action input validated against an enum; returns the task result and post-action status; refuses unknown `vmid` with a clear error.

### [Phase 2] Issue #10: LXC power management tool
- **Description:** `proxmox_lxc_power` — start / stop / shutdown / reboot an LXC container, task-aware.
- **Expected Files Touched:** `internal/features/lxc_power.go`, `internal/features/lxc_power_test.go`, `internal/proxmox/service_lxc_power.go`, `docs/tools/lxc_power.md`.
- **Acceptance Criteria:** Mirrors VM power semantics for containers; distinguishes "already running"/"already stopped"; task completion confirmed.

### [Phase 2] Issue #11: Snapshot tool (create / list / delete)
- **Description:** `proxmox_snapshot` — create a named snapshot (with optional description and `vmstate`), list snapshots, and delete a snapshot, for VMs and containers.
- **Expected Files Touched:** `internal/features/snapshot.go`, `internal/features/snapshot_test.go`, `internal/proxmox/service_snapshot.go`, `docs/tools/snapshot.md`.
- **Acceptance Criteria:** Create returns after the UPID completes; list shows names + timestamps + parent; delete is idempotent on missing snapshots; `current` is never deletable.

### [Phase 2] Issue #12: Snapshot rollback tool
- **Description:** `proxmox_snapshot_rollback` — roll a VM/LXC back to a named snapshot. Kept separate from #11 because it is destructive and warrants its own slice and guardrails.
- **Expected Files Touched:** `internal/features/snapshot_rollback.go`, `internal/features/snapshot_rollback_test.go`, `internal/proxmox/service_snapshot_rollback.go`, `docs/tools/snapshot_rollback.md`.
- **Acceptance Criteria:** Validates the target snapshot exists before rollback; surfaces the running/stopped transition; task-aware; clear error if the guest is locked.

### [Phase 2] Issue #13: System metrics resource
- **Description:** Expose metrics as an MCP **Resource** (not a tool) to exercise the Resource registration path — `proxmox://metrics/node/{node}` and `proxmox://metrics/guest/{vmid}`, backed by `go-proxmox` `RRDData` (CPU, mem, disk, netio) over a timeframe.
- **Expected Files Touched:** `internal/features/metrics.go`, `internal/features/metrics_test.go`, `internal/proxmox/service_metrics.go`, `docs/resources/metrics.md`.
- **Acceptance Criteria:** Resource is listed via `resources/list`; reading a node URI returns recent RRD series; invalid URI yields a clean MCP error; timeframe defaults to hour.

### [Phase 2] Issue #14: LXC creation tool
- **Description:** `proxmox_create_lxc` — create a container from a template (ostemplate, hostname, cores, memory, disk, storage, bridge, optional `start`). Task-aware.
- **Expected Files Touched:** `internal/features/lxc_create.go`, `internal/features/lxc_create_test.go`, `internal/proxmox/service_lxc_create.go`, `docs/tools/lxc_create.md`.
- **Acceptance Criteria:** Required fields enforced via input schema; returns the new `vmid` and task result; optional auto-start honored; rejects an in-use `vmid`.

### [Phase 2] Issue #15 (stretch): Node triage prompt
- **Description:** Register an MCP **Prompt** — `triage_node` — that templates a guided workflow (check node status → list guests → pull metrics → summarize anomalies), exercising the Prompt registration path. Composes existing tools/resources, so it needs **no** `service_*.go` method.
- **Expected Files Touched:** `internal/features/prompt_triage.go`, `internal/features/prompt_triage_test.go`, `docs/prompts/triage_node.md`.
- **Acceptance Criteria:** Prompt appears in `prompts/list`; accepts a `node` argument; renders a coherent multi-step message referencing the real tool names.

### [Phase 2] Issue #16 (stretch): VM clone tool
- **Description:** `proxmox_vm_clone` — clone a VM (full/linked) to a new `vmid` with optional name/target node. Task-aware.
- **Expected Files Touched:** `internal/features/vm_clone.go`, `internal/features/vm_clone_test.go`, `internal/proxmox/service_vm_clone.go`, `docs/tools/vm_clone.md`.
- **Acceptance Criteria:** Full vs linked clone selectable; new `vmid` returned after task completion; rejects clone over an existing `vmid`.

---

## Disjoint blast-radius matrix (sanity check)

| Issue | features file | proxmox file | docs file |
|---|---|---|---|
| #7 nodes | `nodes.go` | `service_nodes.go` | `tools/nodes.md` |
| #8 guests | `inventory.go` | `service_inventory.go` | `tools/inventory.md` |
| #9 vm power | `vm_power.go` | `service_vm_power.go` | `tools/vm_power.md` |
| #10 lxc power | `lxc_power.go` | `service_lxc_power.go` | `tools/lxc_power.md` |
| #11 snapshot | `snapshot.go` | `service_snapshot.go` | `tools/snapshot.md` |
| #12 rollback | `snapshot_rollback.go` | `service_snapshot_rollback.go` | `tools/snapshot_rollback.md` |
| #13 metrics | `metrics.go` | `service_metrics.go` | `resources/metrics.md` |
| #14 lxc create | `lxc_create.go` | `service_lxc_create.go` | `tools/lxc_create.md` |
| #15 prompt | `prompt_triage.go` | — | `prompts/triage_node.md` |
| #16 vm clone | `vm_clone.go` | `service_vm_clone.go` | `tools/vm_clone.md` |

No path appears twice → no two agents can collide on a file. The only shared file any of them imports (`registry`, `service.go`, `server.go`) is **read-only** to Phase 2.
