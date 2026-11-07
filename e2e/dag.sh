#!/bin/sh
# Print one phase of the e2e job graph as a Mermaid flowchart, for the docs.
#
# The full graph from `probe dag --mermaid e2e/test.yml` includes every step
# and is too large to read on a page. This script collapses each job into one
# node and splits the jobs by their dependencies on the create_release job:
#
#   start     jobs create_release depends on, and create_release itself
#             (Phase 0 to 2)
#   verify    create_release and the jobs that depend on it directly, plus
#             the jobs that depend on neither (the health check failure
#             variant) (Phase 3)
#   followup  jobs that depend on create_release through a verify job,
#             with those verify jobs (Phase 4 to 6)
#
# Usage: e2e/dag.sh start|verify|followup
set -eu

phase=${1:-}
case "$phase" in
start | verify | followup) ;;
*)
  echo "usage: $0 start|verify|followup" >&2
  exit 2
  ;;
esac

dir=$(dirname "$0")

probe dag --mermaid "$dir/test.yml" | awk -v phase="$phase" -v root=create_release '
# A job subgraph: keep its id and label, drop its steps.
/^    subgraph / {
  line = $0
  sub(/^    subgraph /, "", line)
  id = substr(line, 1, index(line, "[") - 1)
  label = substr(line, index(line, "[") + 1)
  sub(/\]$/, "", label)
  order[++n] = id
  labels[id] = label
  injob = 1
  next
}
injob && /^    end$/ { injob = 0; next }
injob { next }
# A dependency between jobs.
/^    [^ ]+ --> [^ ]+$/ {
  from[++m] = $1
  to[m] = $3
  next
}
END {
  # Ancestors of root.
  anc[root] = 1
  do {
    changed = 0
    for (i = 1; i <= m; i++) {
      if ((to[i] in anc) && !(from[i] in anc)) { anc[from[i]] = 1; changed = 1 }
    }
  } while (changed)

  # Descendants of root.
  desc[root] = 1
  do {
    changed = 0
    for (i = 1; i <= m; i++) {
      if ((from[i] in desc) && !(to[i] in desc)) { desc[to[i]] = 1; changed = 1 }
    }
  } while (changed)

  # Direct children of root.
  for (i = 1; i <= m; i++) if (from[i] == root) child[to[i]] = 1

  for (k = 1; k <= n; k++) {
    id = order[k]
    if (phase == "start") {
      keep[id] = (id in anc)
    } else if (phase == "verify") {
      keep[id] = (id == root) || (id in child) || (!(id in anc) && !(id in desc))
    } else {
      keep[id] = (id in desc) && id != root && !(id in child)
    }
  }
  # followup also shows the verify jobs it depends on.
  if (phase == "followup") {
    for (i = 1; i <= m; i++) if (keep[to[i]] && (from[i] in child)) keep[from[i]] = 1
  }

  print "flowchart LR"
  for (k = 1; k <= n; k++) {
    id = order[k]
    if (keep[id]) printf "    %s[%s]\n", id, labels[id]
  }
  print ""
  for (i = 1; i <= m; i++) {
    if (keep[from[i]] && keep[to[i]]) printf "    %s --> %s\n", from[i], to[i]
  }
}'
