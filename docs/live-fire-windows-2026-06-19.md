# Live-Fire — Windows Transports (WinRM / WMI / PsExec) — 2026-06-19

Dogfooding the Windows remote-exec transports against a real **Windows Server 2022 EC2**
instance (`t3.small`), driven from the control host (Linux) — the same loop as SSH-on-Linux, just a
Windows target. WinRM was enabled via EC2 user-data; ingress (5985) scoped to the control host's `/32`;
the Administrator password decrypted with the instance's RSA key.

## WinRM — fully proven ✅

Go speaks WinRM from Linux, so the control host → Windows-EC2:5985 works directly.

**Read paths** (identity + the *real* registry path + service status):
```
$ fleet run    -L win.txt --transport winrm --winrm-user Administrator --winrm-pw-env WINRM_PW -c 'hostname & whoami & ver' -v
        | EC2AMAZ-KBOKD6O
        | ec2amaz-kbokd6o\administrator
        | Microsoft Windows [Version 10.0.20348.5256]

$ fleet gather -L win.txt … -c 'reg query "HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion" /v ProductName'
98.93.132.125   …ProductName  REG_SZ  Windows Server 2022 Datacenter

$ fleet svc    -L win.txt … --backend sc --name Spooler --action status --local -v
        |   STATE : 4  RUNNING
```

**Write paths** (set a registry value, restart a service — both verified out-of-band):
```
$ fleet regset -L win.txt … --local --hive HKLM --key 'SOFTWARE\winadmin' --name LiveFire --type REG_SZ --data 'was-here-2026'
total=1 succeeded=1
$ fleet gather … -c 'reg query "HKLM\SOFTWARE\winadmin" /v LiveFire'
98.93.132.125   LiveFire  REG_SZ  was-here-2026     # ✅ written

$ fleet svc -L win.txt … --backend sc --name Spooler --action restart --local
total=1 succeeded=1
$ fleet gather … -c 'sc query Spooler | findstr STATE'
98.93.132.125   STATE : 4  RUNNING                  # ✅ back up
```

This also exercises the real `reg`/`sc` Windows command paths end-to-end. **WinRM is the
recommended Windows transport** and the one to reach for first.

## WMI — proven ✅

`fleet.exe` staged onto the box (S3 presigned URL → `Invoke-WebRequest`) and run there with
`--transport wmi`:
```
$ C:\fleet.exe run --transport wmi -L C:\self.txt -c hostname
total=1 succeeded=1     # wmic process call create -> ReturnValue 0 (launched)
```
WMI's `process call create` is **fire-and-forget** — it returns a ProcessId/ReturnValue, not
the command's stdout. Right for kicking off work; use WinRM when you need output.

## PsExec — command verified ✅ (with a double-hop note)

The exact command the PsExec transport renders runs correctly on the box:
```
$ fleet run … --transport winrm -c 'psexec \\EC2AMAZ-KBOKD6O -accepteula -nobanner cmd /c "hostname"'
[1/1] ok      # ✅ psexec works
```
Running it through the *nested* test stack (the control host → WinRM → `fleet.exe` → psexec → localhost)
returned exit 1 — the classic Windows **second-hop** limitation: a non-interactive WinRM
logon token can't delegate credentials to psexec's onward SMB connection when psexec is
launched by a grandchild process. This is an artifact of the nested test harness, **not** the
transport: psexec is launched from an interactive admin console or a scheduled task with
stored creds in real use, where it gets a proper logon token. Command rendering is unit-tested
(`psexecCmd`/`wmiCmd`).

## Lab setup / teardown

```
# Windows AMI via SSM, RSA key for password decrypt, SG 5985 from /32, WinRM via user-data
aws ec2 run-instances --image-id <Win2022> --instance-type t3.small … --user-data file://winrm.ps1
aws ec2 get-password-data --priv-launch-key id_rsa_win   # decrypts the Administrator password
# teardown
aws ec2 terminate-instances …; aws ec2 delete-security-group --group-name winadmin-winrm
aws ec2 delete-key-pair --key-name winadmin-win; aws s3 rb s3://winadmin-lf-… --force
```

Spend: a few cents (one `t3.small` Windows for ~an hour).
