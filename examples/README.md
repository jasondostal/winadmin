# Examples — screenshot recipes

All of these run against `examples/hosts.sample.txt` (12 fake branch DCs) over the **local**
transport with harmless `echo`/`sleep` commands — nothing leaves your machine. Build first:

```sh
go build -o bin/fleet ./cmd/fleet   # -o fleet would collide with the fleet/ pkg dir
```

### 1. The run builder (TUI form)

```sh
./bin/fleet tui
```
Tab through fields, flip the **Task** selector (`←/→`) to watch the fields change per verb,
toggle **What-if**, then `ctrl+g` to launch.

### 2. The live dashboard — busy, with spinners + a failure + waves

A staggered command (random sleep) keeps spinners moving; `BR05` is rigged to fail:

```sh
./bin/fleet run -L examples/hosts.sample.txt -P 4 \
  -c 'sleep $((RANDOM % 3)); test "{{.Name}}" != "BR05-DC01"' --tui
```

Add a staged rollout to capture the **canary/wave** marker in the header:

```sh
./bin/fleet run -L examples/hosts.sample.txt -P 3 --canary 1 --wave 3 --health-cmd 'true' \
  -c 'sleep $((RANDOM % 2)); echo ok {{.Name}}' --tui
```

### 3. The gather results table (filterable)

```sh
./bin/fleet gather -L examples/hosts.sample.txt \
  -c 'echo "$(hostname) load=$(uptime | sed "s/.*load//")"' --tui
```
Type to filter; `↑/↓` scroll; `esc` quits.

> Tip for crisp screenshots: a ~120×32 terminal, a dark theme, and a font with good box-drawing
> glyphs (the borders + spinners render best there).
