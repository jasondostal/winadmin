# Live-Fire — Provisioning + Status Board — 2026-06-20

Dogfooding the **provisioning arc** end-to-end against a real, heterogeneous Windows fleet:
`provision` → agent-installed-as-a-service → file registry → live `status` board — all driven
from a Linux control host over WinRM.

**The lab:** 18 Windows EC2 instances (`t3.medium`) — 12× **Server 2022**, 3× **Server 2019**,
3× **Server 2016** — a genuinely mixed fleet, each box reporting its own OS. WinRM (5985) enabled
via EC2 user-data, ingress scoped to the control host's `/32`, Administrator password set in
user-data. The agent (`fleet.exe`, cross-compiled `windows/amd64`) was staged in S3; the control
host ran the Linux `fleet` binary. Everything was torn down to **$0** afterward.

---

## What was proven ✅

### `fleet provision` — install the agent as a service, fleet-wide

Per box, over WinRM: download `fleet.exe`, `sc.exe create`/`start` the service, register it. The
whole install ships as a single UTF-16 **base64 `-EncodedCommand`** PowerShell payload, so there's
no cmd/`sc.exe` quoting to fight, wrapped in try/catch.

```
provision :: 18 target(s) over winrm — install service "fleetagent"
[1/18] … ok
…
provisioned 18/18 — registry: fleet-registry.json
```

### agent-as-a-real-service

`fleet agent`, when started by the Windows SCM, runs under the service-control protocol
(`golang.org/x/sys/windows/svc`) — so `sc create` yields a **real, managed service**, not a
process that the SCM kills after 30 s. Confirmed `RUNNING` on every box via `sc query`.

### the file registry + the live status board

`provision` captured each box's **hostname** (`$env:COMPUTERNAME`) and **OS caption**
(`Win32_OperatingSystem`) into a JSON registry. `fleet status` then polls the agent service across
the registry on a cycle — the engine's fan-out *is* the poller. `--tui` renders the live board:
hostname + OS + state + per-box response **latency**, problems on top, a per-OS roll-up.

```
==== fleet status ====
  …  EC2AMAZ-…  Windows Server 2022 Datacenter  RUNNING   88ms
  ---- 18/18 running ----
```

The **two-sessions-one-fleet** property fell out for free: any `fleet status` pointed at the same
registry file sees the same fleet. Put the file on a share and it's the whole team's view — no
server, no DB.

---

## Bugs the live-fire caught (the point of live-fire)

None of these would have shown up in a unit test or a cross-compile — exactly the "looks fine,
broken at the seams" failures that only a real target surfaces:

1. **cmd.exe's 8,191-char command limit.** Presigned S3 URLs (~1,400 chars each, fat session
   token) made the base64 payload overflow cmd's line limit → it was silently truncated →
   PowerShell got garbage → fast-fail with empty output. **Fix:** short public URLs.
2. **A `--loop` flag collision.** `status` redefined `--loop`, which the lifecycle work had since
   added to the shared flag set → hard panic on first run. **Fix:** reuse the common `--loop`.
3. **Non-idempotent re-provision.** Re-running failed because the *running* service held
   `fleet.exe` locked, so the download couldn't overwrite it. **Fix:** stop+delete the service
   (and wait for it to clear) **before** downloading.
4. **Status refresh wiped fields.** The status `Upsert` replaced the whole machine record, blanking
   OS/hostname after the first poll (the board showed "unknown"). **Fix:** `Upsert` preserves
   fields the caller didn't set.
5. **A meaningless "last seen" column.** Because every box polls together, "Xs ago" was identical
   across all rows and just oscillated `0..interval` — noise dressed as data. **Fix:** show
   per-box **poll latency** instead (a real, varying health signal); keep `last_seen` in the
   registry for cross-session staleness.

---

## Teardown

All 18 instances terminated, security group and S3 bucket deleted, verified at **$0**. The lab
left no lingering resources.
