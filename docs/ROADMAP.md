# winadmin / fleet Roadmap

The engine is a **"do X to N targets, safely, with a dry-run and an audit trail"** machine.
`X` is pluggable, so this roadmap is really a backlog of high-value `X`s plus the safety and
reporting layers that make fleet ops *humane* instead of just powerful.

Legend: ✅ done · 🚧 in progress · ⬜ planned

## Foundation (done)

- ✅ `fleet` engine — bounded worker pool, per-target timeout,
  `--what-if` dry-run, `--stop-on-error`, structured `slog` audit, completion summary.
- ✅ Transports — `local` (remote-admin model: `reg \\host`, `robocopy \\host`) and `ssh`
  (run on the box; key + agent auth, host-key verification, custom port).
- ✅ Dynamic inventory — `-L file` or `--inventory-cmd '<query>'` (AD group, cloud API, OU).
- ✅ Tasks — `run` (templated command), `regset`, `deldir`, `ldapset`.
- ✅ TUIs — live "btop-style" dashboard + interactive run builder.
- ✅ Proven in live-fire — 10× EC2 over real SSH; AD attribute set across an OU.

## Actions — the "write" verbs

- ✅ **`svc`** — start/stop/restart/status a service. `sc \\host` (Windows) · `systemctl`
  (+`--sudo`, ssh/Linux). **Live-fired** (EC2 chronyd restart).
- ✅ **`install`** — silent install. `msiexec /i "<pkg>" /qn` · `setup.exe /S` · `sh`.
  **Live-fired** (dnf install across EC2).
- ✅ **`push`** — copy to many machines. `robocopy … /MIR` · `scp`/`rsync`. (Rendering +
  unit-tested; robocopy form is Windows.)
- ✅ **`reboot`** — `shutdown /r /m \\host` · linux `shutdown -r`. Guarded by `--yes`.
- ✅ **`proc`** — kill a process: `taskkill /s host /im` · `pkill`.
- ✅ **`task`** — scheduled tasks: run/delete/query/create across machines (`schtasks /s host`).
- ✅ **`localgroup`** — add/remove a member of a local group (`net localgroup … /add`).
- ✅ **`firewall`** — add/delete firewall rules (`netsh advfirewall …`).

## Reading — the "report" side (the sleeper hit)

- ✅ **`gather`** — run a query per host and **aggregate** stdout into table / CSV / JSON.
  **Live-fired** (kernel+uptime table across EC2). The rest of this entry, for reference:
  keyed by target. Turns the tool from write-only into a fleet **reporting engine**:
  "which boxes still have the old DLL version?", "disk free everywhere", "who's logged on",
  "service X state across the fleet". The engine already collects per-target stdout; gather
  just tabulates and lets you filter/sort.

## Safety — what makes it *good*, not just powerful

- ✅ **Staged rollout** — `--canary N`, `--wave M`, `--health-cmd '<check>'`, `--pause`.
  **Live-fired** both ways on EC2 (healthy canary→waves; health-gate abort skips the rest).
  The difference between "I patched the fleet" and "I took down the fleet."
- ✅ **Per-target retries** — `--retries N --retry-backoff` for flaky transports/hosts.
- ✅ **Result export** — `--export results.json|results.csv` writes the full per-target set.
- ✅ **Confirm-on-blast** — destructive verbs (`deldir`, `reboot`, `proc`) refuse >1 target
  without `--yes` (or `--what-if`).
- ✅ **Target filter / preview** — `--match 'web*,db0?'` keeps matching targets; `--preview`
  prints the resolved list and exits without running.
- ✅ **Lifecycle** — `--loop N` (`0` = forever), `--wait`/`--start-at` to schedule the start,
  and `--pre`/`--post` to bracket the fan-out with a control-host command. Full run-model
  parity with the old overnight fleet-runners, with the TUI showing countdown/loop/pre-post.

## Transports / connectivity — reach more fleets

Two connectivity models exist today, both authentic: the **remote-admin model** (the `local`
transport, where the *command* reaches the box — `sc \\host`, `reg \\host`, `schtasks /s`,
`taskkill /s`, `robocopy \\host` — over Windows RPC/SMB) and the **on-the-box model** (`ssh`).
The remaining methods, roughly in priority order for a Windows shop. All Windows transports
are **live-fired on a Windows Server EC2** target (driven from a Linux control host), same dogfood pattern
as SSH-on-Linux — so these are buildable *and* testable, not blocked:

- ✅ **WinRM transport** — `WinRMTransport` over `masterzen/winrm`; `--transport winrm`.
  **Live-fired** on Windows Server 2022 EC2 from Linux: identity, registry read+write,
  service status+restart (see `docs/live-fire-windows-2026-06-19.md`). The recommended
  Windows transport.
- ✅ **PsExec transport** — `PsExecTransport` (`psexec \\host -u … -p …`); `--transport psexec`.
  Command rendering unit-tested + verified working on a live Windows box. (Full nested
  live-fire hit the known Windows double-hop limit; works from an interactive/scheduled
  context.)
- ✅ **WMI transport** — `WMITransport` (`wmic /node:host process call create`);
  `--transport wmi`. **Live-fired** (fire-and-forget exec; ReturnValue 0).
- ✅ **Agent / pull model** — `fleet agent --source <file|url> --interval` pulls a job and
  runs it only when its version changes (a robocopy /MIR-style pull pattern). **Live-fired** on Linux.
- ❌ **RDP** — interactive only; out of scope for automation.
- ✅ **SSH password auth via `secret`** — `--ssh-pw-env` wires the `secret` ladder
  (env → DPAPI → CredMan) into the SSH transport (`SSHTransport.PasswordProvider`).
- ✅ **Inventory plugins** — `--inventory file:<p> | cmd:<sh> | aws:<filter> | ad-ou:<dn> |
  ad-group:<dn>` over the generic `--inventory-cmd` (ad-* read LDAP_URL/LDAP_BIND_DN/LDAP_PW).

## TUI

- ✅ **Verb-aware run builder** — the console speaks every verb (`run`, `svc`, `install`,
  `push`, `reboot`, `proc`, `regset`, `deldir`, `task`, `localgroup`, `firewall`, `ldapset`)
  with verb-specific fields, SSH transport fields, and the staging fields. Parity-tested.
- ✅ **Staged rollout in the dashboard** — the live Watcher runs `RunWaves` when staging is
  set (canary→waves with the health gate), driven from the builder.
- ✅ **Wave view** — the dashboard header shows the current canary/wave (`▶ CANARY 1/4`) as
  batches advance.
- ✅ **gather view** — `fleet gather --tui` opens a spreadsheet over the per-target output:
  it parses the query's stdout into real columns (`kv` / `columns` / `csv` hints), and you can
  **sort by any column** (numeric-aware — `5G` < `20G`), **group into an expandable tree by any
  column** (synthetic `TARGET`/`EXIT`, an `OS` column injected from the registry, or anything the
  query parses out), and `/`-filter across all cells. It's "just data," spreadsheet-style.
- ✅ **gather query library** — a config-backed set of canned queries (free disk, OS version,
  uptime, who's logged on, …) surfaced as a picker in the run builder and resolvable by name on
  the CLI (`fleet gather --query 'free disk (linux)'`, `--list-queries`). Built-ins ship by
  default; override via the `gather_queries` config key.
- ✅ **gather in the run builder** — `gather` is a first-class verb in the console; picking it
  hands off to the spreadsheet view (it runs the query itself, then lets you sort/group).
- ✅ **Consistent read-only keybinds** — the dashboard, status board, and gather view all quit on
  `q`/`esc`/`ctrl+c`; gather reserves bare keys for sort/group and enters filtering with `/`.

## CI / Release / supply chain

- ✅ **Cross-platform release builds in CI** — `.github/workflows/release.yml` builds `fleet`
  for windows/amd64, linux/amd64, darwin/arm64+amd64 on tag and attaches them to the Release.
- ✅ **SBOM in the pipeline** — release workflow generates a CycloneDX SBOM
  (`cyclonedx-gomod`) and attaches `fleet.cyclonedx.json` per release.
- ✅ **Provenance + signing** — `SHA256SUMS` + keyless **cosign** signature
  (`SHA256SUMS.sig`/`.pem` via Sigstore OIDC) + **SLSA build-provenance attestation**
  (`actions/attest-build-provenance`) on every release binary.
- ✅ **CI gates** — `.github/workflows/ci.yml`: gofmt, `go vet`, `go test`, `govulncheck`,
  and `GOOS=windows go vet` on every push/PR.
- ✅ **`windows-latest` runner** — `.github/workflows/ci.yml` runs `go test ./...` on a real
  Windows runner every push, so the registry/runas/groups/WinRM paths actually *execute* in
  CI — closing the "compile-verified but not run" gap from the primer.

## Testing posture

Every feature gets a **live-fire** pass against a throwaway lab, not just unit tests —
that's how the SSH-auth, `--ssh-port`, `-v`, and dynamic-inventory gaps were caught.
Findings land in `docs/live-fire-*.md`. Lab matrix:

- **SSH / Linux verbs** → Docker `sshd` containers + Linux EC2 (systemd, WAN). ✅ done.
- **LDAP / AD inventory** → OpenLDAP container. ✅ done.
- **Windows transports (WinRM/PsExec/WMI) and reg/sc paths** → **Windows Server 2022 EC2**
  (t3.small, WinRM via user-data, ingress scoped to a `/32`), driven from the control host. ✅ done
  2026-06-19 — see `docs/live-fire-windows-2026-06-19.md`.
- **CI** → `windows-latest` GitHub runner executes the Windows-tagged tests every push.
