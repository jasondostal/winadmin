#!/usr/bin/env bash
# Example "batch": do several things to each folder passed in ($1).
set -eu
dir="$1"
[ -f "$dir/old.log" ] && mv "$dir/old.log" "$dir/archived.log"
printf 'processed %s\n' "$dir" > "$dir/marker.txt"
echo "[$dir] archived old.log, wrote marker"
