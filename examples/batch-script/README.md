# Batch a script across a list — of *anything*

The oldest fleet-runner trick: write a little script that does several things, then
run it once per item in a list. The list isn't only machines — it's whatever you
put in it. **Folders. Usernames. Files. Tickets.** Each item is handed to your
script (or templated into a verb), and the whole fan-out gets fleet's brakes:
`--what-if`, `-P`, staged `--canary/--wave`, `--retries`, `--export`, an audit log.

## 1. A script per item — the item as `$1` (`%1`)

`--script` runs a local script/batch once per list row, passing the row as its
argument. A whitespace row arrives split, so the script sees `$1 $2 $3`:

```sh
# folders.sample.txt is a list of directories, not hosts
fleet run -L folders.sample.txt --script ./rename-folder.sh --what-if
fleet run -L folders.sample.txt --script ./rename-folder.sh        # for real

# a row like "jdoe Tellers logon_v2.bat" -> $1=jdoe $2=Tellers $3=logon_v2.bat
fleet run -L users.sample.txt --script ./user-batch.sh
```

(`--script ./x.sh` is shorthand for `-c './x.sh {{.Name}}'`.)

## 2. Named columns, templated into *any* verb

Split each row into columns and name them, then reference `{{.<name>}}` — not just
in `run`, but in any verb's flags. fleet resolves them per row:

```sh
# users.csv rows: "jdoe,Tellers,logon_v2.bat"
fleet run -L users.csv --csv --cols user,group,script \
  -c 'set-logon {{.user}} {{.script}}' --what-if

fleet localgroup -L users.csv --csv --cols user,group \
  --member '{{.user}}' --group '{{.group}}' --action add --what-if
```

Columns also work positionally with no setup — `{{.F1}}`, `{{.F2}}`, … come from
splitting each row on whitespace (`--delim`/`--csv` to change the split). The whole
row is always `{{.Name}}`.

## Files here
- `rename-folder.sh` — example "batch": archive a log + drop a marker, per folder.
- `folders.sample.txt` — a list of folders.
- `user-batch.sh` — example multi-column batch (`$1 $2 $3`).
- `users.sample.txt` — whitespace rows; `users.csv` shows the comma form.

> Always dry-run first with `--what-if`. These run with whatever rights you have —
> and history shows that used to be *a lot*.
