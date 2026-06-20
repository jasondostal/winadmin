# Live-Fire — the gather spreadsheet (columns / sort / group / query library) — 2026-06-20

Dogfooding the new `gather --tui` **spreadsheet** — parse each box's output into real
columns, then sort/group/filter by any of them — against three real labs:

- **8-box mixed-Linux** lab (Docker `sshd` containers: Ubuntu 24.04/22.04, Debian 12/11,
  Rocky 9.3, Alma 9.8) over SSH, driven from a Linux control host.
- a single **Windows Server 2022** (Azure) over WinRM, and then
- a **4-version Windows Server fleet** (EC2: **2016 / 2019 / 2022 / 2025**) over WinRM —
  the one that earned its keep (see the wmic finding below).

The point of the tool is a fleet of *Windows* boxes; proving the columnar gather on Linux first
was convenient, but the Windows fleet is where the real lessons came from.

## What we proved ✅

**Linux — group by OS, numeric-aware sort, OS injected from the registry:**
```
 fleet — gather   8/8 rows  sort VERSION_ID▲  ·  group NAME
  TARGET     OS           NAME             VERSION_ID▲ VERSION_CODENAME
▾ AlmaLinux  (1)
  …          Alma 9.8     AlmaLinux        9.8
▾ Debian GNU/Linux  (2)
  …          Debian 11    Debian GNU/Linux 11          bullseye
  …          Debian 12    Debian GNU/Linux 12          bookworm
▾ Rocky Linux  (2)
  …          Rocky 9.3    Rocky Linux      9.3
▾ Ubuntu  (3)
  …          Ubuntu 22.04 Ubuntu           22.04       jammy
  …          Ubuntu 24.04 Ubuntu           24.04       noble
```

**Windows — group by OS across four Server versions (real CIM output):**
```
 fleet — gather   4/4 rows  sort BuildNumber▲  ·  group OS
  TARGET         OS                  EXIT Caption                          Version    BuildNumber▲
▾ Windows Server 2016  (1)
  34.228.254.161 Windows Server 2016 0    Microsoft Windows Server 2016 Da…10.0.14393 14393
▾ Windows Server 2019  (1)
  34.229.15.37   Windows Server 2019 0    Microsoft Windows Server 2019 Da…10.0.17763 17763
▾ Windows Server 2022  (1)
  54.167.97.125  Windows Server 2022 0    Microsoft Windows Server 2022 Da…10.0.20348 20348
▾ Windows Server 2025  (1)
  34.229.167.65  Windows Server 2025 0    Microsoft Windows Server 2025 Da…10.0.26100 26100
```

**Windows — free disk, raw bytes humanized for display, sorted on the true value:**
```
 fleet — gather   4/4 rows  sort FreeBytes▼
TARGET         OS                  EXIT DeviceID FreeBytes▼ SizeBytes
34.228.254.161 Windows Server 2016 0    C:       10.6G      30.0G
54.167.97.125  Windows Server 2022 0    C:       10.4G      30.0G
34.229.15.37   Windows Server 2019 0    C:       9.6G       30.0G
34.229.167.65  Windows Server 2025 0    C:       7.9G       29.5G
```

## Lessons the dogfooding taught us

### 1. `wmic` is gone on Windows Server 2025 🔥 (the headline)
The first three boxes returned OS/disk data; **2025 came back empty.** `where wmic` finds
nothing on Server 2025 — Microsoft has removed the deprecated `wmic` — while `Get-CimInstance`
works on every version. The canned Windows queries had been `wmic …`; they failed *silently*
(empty output, exit 0) on 2025.

**Fix:** the default Windows query library now uses **PowerShell/CIM** (`Get-CimInstance … |
ConvertTo-Csv -NoTypeInformation`), which works 2016→2025. This only surfaced because we ran a
*version-varied* fleet — a single 2022 box would have shipped the bug.

### 2. `ConvertTo-Csv` double-quotes everything; wmic emits `\r\r\n`
Real Windows CSV is `"Caption","Version"` (every field quoted), and wmic output uses **double
carriage returns** (`\r\r\n`). The first parser split naively on commas and would have leaked
quotes and `\r` into cells. **Fix:** `parseCSV` now runs on stdlib `encoding/csv` (handles
quoting + embedded commas), after `splitLines` strips the `\r`/`\r\r`. Verified: no quote/CR
leaks into any cell on real output.

### 3. Column order was non-deterministic (caught before it shipped)
The first columnar build derived column order from Go **map iteration** — so columns reordered
between runs. Caught by an offline render of real `os-release` output (a column visibly jumped
position). **Fix:** parsers now return header order explicitly; a regression test builds the
table 50× and asserts a stable order.

### 4. The sort marker (`▲/▼`) truncated the header
The pad helpers measured **bytes**, so the multi-byte arrow overflowed its column and got
clipped (`Usepct…`). **Fix:** `trunc`/`padRight`/`padLeft` are now rune-aware (a no-op for the
ASCII they're usually fed, correct for the glyph).

### 5. A nicety, not a bug: failure triage via `group EXIT`
When the fleet is unreachable (e.g. after teardown), every target errors and gather drops the
WinRM/SSH error into an `OUTPUT` cell with `EXIT -1`. Grouping by `EXIT` then collapses all the
failures into one bucket — a quick "what's broken across the fleet" view that fell out of the
"it's just data" model for free.

### Known wart
On a **descending** sort, blank cells (e.g. a CD-ROM's empty `FreeSpace`) bubble to the top,
because blanks sort as text and text sorts after numbers ascending. Defensible ("unknown" floats
up); could be changed to force blanks last if it grates.

## Lab setup / teardown

```
# Linux: 8 Docker sshd containers across 5 distros, on a docker bridge; driven over SSH.
# Windows: EC2 run-instances per version (SSM AMIs Windows_Server-{2016,2019,2022,2025}),
#   WinRM via user-data (Enable-PSRemoting + Basic/AllowUnencrypted), SG 5985 from the /32.
fleet gather -L hosts.txt --transport winrm --winrm-user Administrator --winrm-pw-env WINRM_PW \
  --query 'os version (windows)' --registry registry.json --tui
# teardown: aws ec2 terminate-instances …(tag winfleet=1); delete SG; az group delete fleet-winlab
```

Spend: a handful of `t3.medium` Windows instances + one small Azure VM for ~an hour — a few
dollars, torn down to $0.
