#!/usr/bin/env bash
# Example multi-column batch: a whitespace row arrives as $1 $2 $3.
user="$1"; group="${2:-}"; logon="${3:-}"
echo "[$user] net localgroup '$group' '$user' /add  ;  set logon-script -> $logon"
