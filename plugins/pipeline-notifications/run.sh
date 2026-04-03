#!/usr/bin/env bash
set -euo pipefail

# Pipeline Notifications — ntfy.sh subscriber
#
# Spawns a persistent curl connection to ntfy.sh/beacon-ci/json.
# Each event is parsed and forwarded to the mayor via gt mail.
# Idempotent: skips if watcher already running.

PLUGIN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PIDFILE="$PLUGIN_DIR/.watcher.pid"
LOGFILE="$PLUGIN_DIR/.watcher.log"
HANDLER="$PLUGIN_DIR/handle-event.sh"

# If watcher already running, report and exit
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  echo "Pipeline notification watcher already running (PID $(cat "$PIDFILE"))"
  bd create "pipeline-notifications: already running" -t chore --ephemeral \
    -l type:plugin-run,plugin:pipeline-notifications,result:success \
    -d "Watcher already running (PID $(cat "$PIDFILE"))" --silent 2>/dev/null || true
  exit 0
fi

# Verify gt mail is available
if ! command -v gt &>/dev/null; then
  echo "FATAL: gt not found in PATH"
  exit 1
fi

# Start the persistent subscriber in the background
# ntfy.sh/beacon-ci/json streams one JSON object per line per event.
# --no-buffer ensures line-by-line processing (no curl buffering).
nohup bash -c '
  while true; do
    curl -sfN "https://ntfy.sh/beacon-ci/json" 2>/dev/null | while IFS= read -r line; do
      [ -z "$line" ] && continue

      # Parse the JSON event
      EVENT_TYPE=$(echo "$line" | jq -r ".event // empty" 2>/dev/null)

      # Only process "message" events (skip keepalives, open events)
      if [ "$EVENT_TYPE" != "message" ]; then
        continue
      fi

      TITLE=$(echo "$line" | jq -r ".title // .topic // \"CI Event\"" 2>/dev/null)
      MESSAGE=$(echo "$line" | jq -r ".message // empty" 2>/dev/null)
      PRIORITY=$(echo "$line" | jq -r ".priority // 3" 2>/dev/null)
      TAGS=$(echo "$line" | jq -r "(.tags // []) | join(\", \")" 2>/dev/null)
      CLICK_URL=$(echo "$line" | jq -r ".click // empty" 2>/dev/null)
      TIMESTAMP=$(echo "$line" | jq -r ".time // empty" 2>/dev/null)

      # ── BEAD_STATUS handler ──────────────────────────────────────
      # CI sends: Title: "BEAD_STATUS: be-xxx to <status>"
      #           Body:  "bd update be-xxx --status <status>"
      # We parse the title and execute bd commands to update Dolt.
      if [[ "$TITLE" == BEAD_STATUS:* ]]; then
        # Extract bead ID and target status from title
        BEAD_ID=$(echo "$TITLE" | grep -oE "be-[a-z0-9]+" | head -1)
        TARGET_STATUS=$(echo "$TITLE" | sed "s/.*to //" | tr -d "[:space:]")

        if [ -n "$BEAD_ID" ] && [ -n "$TARGET_STATUS" ]; then
          case "$TARGET_STATUS" in
            closed)
              bd close "$BEAD_ID" 2>/dev/null && \
                echo "[$(date -Iseconds)] BEAD_STATUS: closed $BEAD_ID" >> "'"$LOGFILE"'" || \
                echo "[$(date -Iseconds)] BEAD_STATUS: failed to close $BEAD_ID" >> "'"$LOGFILE"'"
              bd label remove "$BEAD_ID" gt:slung 2>/dev/null || true
              bd label remove "$BEAD_ID" gt:deploying 2>/dev/null || true
              ;;
            deploying)
              bd update "$BEAD_ID" --status deploying 2>/dev/null && \
                echo "[$(date -Iseconds)] BEAD_STATUS: $BEAD_ID -> deploying" >> "'"$LOGFILE"'" || \
                echo "[$(date -Iseconds)] BEAD_STATUS: failed to update $BEAD_ID to deploying" >> "'"$LOGFILE"'"
              bd label add "$BEAD_ID" gt:deploying 2>/dev/null || true
              ;;
            *)
              bd update "$BEAD_ID" --status "$TARGET_STATUS" 2>/dev/null && \
                echo "[$(date -Iseconds)] BEAD_STATUS: $BEAD_ID -> $TARGET_STATUS" >> "'"$LOGFILE"'" || \
                echo "[$(date -Iseconds)] BEAD_STATUS: failed to update $BEAD_ID to $TARGET_STATUS" >> "'"$LOGFILE"'"
              ;;
          esac
        else
          echo "[$(date -Iseconds)] BEAD_STATUS: parse failed: $TITLE" >> "'"$LOGFILE"'"
        fi
        continue
      fi

      # ── Standard CI notification → mail to mayor ─────────────────
      # Determine severity from ntfy priority (1-5)
      SEVERITY="info"
      case "$PRIORITY" in
        5|4) SEVERITY="FAILURE" ;;
        3)   SEVERITY="SUCCESS" ;;
        2|1) SEVERITY="info" ;;
      esac

      # Build mail body
      MAIL_BODY="$MESSAGE"
      [ -n "$TAGS" ] && MAIL_BODY="$MAIL_BODY\nTags: $TAGS"
      [ -n "$CLICK_URL" ] && MAIL_BODY="$MAIL_BODY\nURL: $CLICK_URL"
      [ -n "$TIMESTAMP" ] && MAIL_BODY="$MAIL_BODY\nTimestamp: $(date -r "$TIMESTAMP" 2>/dev/null || echo "$TIMESTAMP")"

      # Send to mayor
      printf "%b" "$MAIL_BODY" | gt mail send mayor/ -s "CI [$SEVERITY]: $TITLE" --stdin 2>/dev/null || \
        echo "[$(date -Iseconds)] Failed to mail mayor: $TITLE" >> "'"$LOGFILE"'"

      echo "[$(date -Iseconds)] Forwarded: [$SEVERITY] $TITLE" >> "'"$LOGFILE"'"
    done

    # If curl exits (network drop), wait and reconnect
    echo "[$(date -Iseconds)] Connection lost, reconnecting in 10s..." >> "'"$LOGFILE"'"
    sleep 10
  done
' >> "$LOGFILE" 2>&1 &

WATCHER_PID=$!
echo "$WATCHER_PID" > "$PIDFILE"
echo "Pipeline notification watcher started (PID $WATCHER_PID)"

# Record success
bd create "pipeline-notifications: watcher started" -t chore --ephemeral \
  -l type:plugin-run,plugin:pipeline-notifications,result:success \
  -d "ntfy.sh subscriber started (PID $WATCHER_PID)" --silent 2>/dev/null || true
