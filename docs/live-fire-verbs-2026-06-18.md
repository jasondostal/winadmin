# Live-Fire — Fleet Verbs + Staged Rollout (2026-06-18)

Dogfooding the new action verbs and the staged-rollout safety layer against **6 real EC2
`t2.micro` instances** (Amazon Linux 2023, systemd) over real SSH from the control host.

All verbs proven against live, separate machines. The Windows-only forms (`sc`, `msiexec`,
`robocopy`, `taskkill`, `shutdown`) are covered by `--what-if` rendering + unit tests; the
Linux/SSH forms ran for real here.

## `svc` — service control

```
$ fleet svc -L hosts2.txt --transport ssh … --backend systemctl --name chronyd --action status -v
[1/6] 100.52.221.89   ok
        | active                         # is-active across all 6
…
$ fleet svc … --name chronyd --action restart --sudo
total=6  succeeded=6  failed=0  skipped=0   # real `sudo systemctl restart` on every box
```

Found & fixed live: `systemctl start/stop/restart` needs root over SSH → added `--sudo`.

## `gather` — fleet reporting (the read side)

```
$ fleet gather -L hosts2.txt --transport ssh … -c 'echo "$(uname -r) up=$(cut -d. -f1 /proc/uptime)s"'
13.218.246.8   6.1.174-217.345.amzn2023.x86_64 up=105s
54.146.251.85  6.1.174-217.345.amzn2023.x86_64 up=98s
…
```

One query, one table across the fleet. `--format csv|json` for piping into a ticket or a
dashboard. This is the verb that turns the tool from write-only into a reporting engine.

## `install` — silent install

```
$ fleet install -L hosts2.txt --transport ssh … --kind sh --package 'sudo dnf install -y -q' --args cowsay
total=6  succeeded=6  failed=0  skipped=0
# verified out-of-band: `cowsay -V` returns a cow on all 6
```

Real package install on every box. The `msi`/`exe` kinds render `msiexec /i … /qn` /
`setup.exe /S` for Windows targets (the classic msiexec /qn install pattern).

## Staged rollout — the safety layer

Healthy path — canary, then waves, health gate between each:

```
$ fleet svc … --action restart --sudo --canary 1 --wave 2 --health-cmd 'true' -P 2
== CANARY: batch 1/4 (1 target(s)) ==
== WAVE 2: batch 2/4 (2 target(s)) ==
== WAVE 3: batch 3/4 (2 target(s)) ==
== WAVE 4: batch 4/4 (1 target(s)) ==
total=6  succeeded=6  failed=0  skipped=0
```

Abort path — the health gate fails right after the canary, so the rollout stops and the
**rest are never touched**:

```
$ fleet run … -c 'true' --canary 1 --wave 2 --health-cmd 'false'
== CANARY: batch 1/4 (1 target(s)) ==
!! rollout aborted: health check failed after canary: exit status 1
total=6  succeeded=1  failed=0  skipped=5
```

This is the difference between "I restarted a service on one box, confirmed it, then let it
ripple" and "I restarted a broken config across 350 boxes at once." In real use `--health-cmd`
is a probe (HTTP check, `systemctl is-active` against the canary, a synthetic transaction).

## Teardown

```
aws ec2 terminate-instances --instance-ids <6 ids>
aws ec2 delete-security-group --group-name winadmin-lf2
aws ec2 delete-key-pair --key-name winadmin-livefire
```

Spend: ~$0.01 (six `t2.micro` for a few minutes).
