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

      # POPANDPEEK-FORK BEGIN: hq-kkata deacon BEAD_STATUS handler + live-drift reconciliation.
      # Three cases are added below. The first two (witness routing + Released auto-restart)
      # were hand-edited directly into the live watcher copy but never committed back to the
      # gastown source — this block reconciles them. The third (BEAD_STATUS deacon auto-flip)
      # is the primary hq-kkata fix: read ntfy BEAD_STATUS events and write bd update directly,
      # removing witness claude cognition from the hot path. Research bead: be-xt1hj.
      # Root-cause bead: be-oe0vb. Evidence: witness processed mails at 00:29-00:57 UTC yesterday
      # but Dolt status did not flip until mayor manually batch-closed 14h12m later.
      case "$TITLE" in
        "BEAD_STATUS: "*" to "*)
          # POPANDPEEK-FORK: hq-kkata deacon auto-flip — parse the title, look up current
          # bead status, and flip via bd update. Idempotent: skip if current already matches
          # and never flip backwards from terminal states. This is the hq-kkata fix.
          BEAD_ID=$(echo "$TITLE" | grep -oE "\b[a-z]+-[a-z0-9]+" | head -1)
          NEW_STATUS=$(echo "$TITLE" | sed -n "s/.* to \([a-z_]*\).*/\1/p")
          PR_NUM=$(echo "$TITLE" | grep -oE "PR #[0-9]+" | grep -oE "[0-9]+" || true)
          if [ -n "$BEAD_ID" ] && [ -n "$NEW_STATUS" ]; then
            CURRENT=$(bd show "$BEAD_ID" --json 2>/dev/null | jq -r ".[0].status // empty" 2>/dev/null || true)
            if [ -z "$CURRENT" ]; then
              echo "[$(date -Iseconds)] Deacon skip: $BEAD_ID not found" >> "'"$LOGFILE"'"
            elif [ "$CURRENT" = "$NEW_STATUS" ]; then
              echo "[$(date -Iseconds)] Deacon skip: $BEAD_ID already $NEW_STATUS" >> "'"$LOGFILE"'"
            elif [ "$CURRENT" = "closed" ] || [ "$CURRENT" = "deferred" ]; then
              echo "[$(date -Iseconds)] Deacon skip: $BEAD_ID terminal state $CURRENT, not flipping to $NEW_STATUS" >> "'"$LOGFILE"'"
            else
              NOTE="deacon auto-flip: $CURRENT to $NEW_STATUS via ntfy (PR #${PR_NUM:-?})"
              if bd update "$BEAD_ID" --status "$NEW_STATUS" --append-notes "$NOTE" 2>/dev/null; then
                echo "[$(date -Iseconds)] Deacon flipped: $BEAD_ID $CURRENT to $NEW_STATUS (PR #${PR_NUM:-?})" >> "'"$LOGFILE"'"
              else
                echo "[$(date -Iseconds)] Deacon FAILED: bd update $BEAD_ID to $NEW_STATUS" >> "'"$LOGFILE"'"
              fi
            fi
          else
            echo "[$(date -Iseconds)] Deacon skip: could not parse bead_id or status from title" >> "'"$LOGFILE"'"
          fi
          ;;
      esac

      # POPANDPEEK-FORK: witness routing — belt-and-suspenders backup for the deacon handler
      # above, per emmett be-oe0vb backward-compat note. Retired in hq-eovky (follow-up) once
      # the deacon handler has soaked. Restored from live-vs-source drift.
      case "$TITLE" in
        BEAD_STATUS:*|"PR merged:"*|"PR Validation | Passed"*)
          WITNESS_ADDR="beacon/witness"
          printf "%b" "$MAIL_BODY" | gt mail send "$WITNESS_ADDR" -s "CI [$SEVERITY]: $TITLE" --stdin 2>/dev/null || \
            echo "[$(date -Iseconds)] Failed to mail witness: $TITLE" >> "'"$LOGFILE"'"
          echo "[$(date -Iseconds)] Routed to witness: $TITLE" >> "'"$LOGFILE"'"
          ;;
        "Released "*|"REVERT: Released "*)
          # POPANDPEEK-FORK: auto-restart beacon FE/BE on a new release tag. Kill BEFORE pull
          # because Vite HMR crashes if source files change under it mid-run. Restored from
          # live-vs-source drift — this case existed in the running watcher but was never
          # committed back to the gastown source.
          RELEASE_VERSION=$(echo "$TITLE" | grep -oE "v[0-9]+\.[0-9]+\.[0-9]+" | head -1 || echo "unknown")
          echo "[$(date -Iseconds)] Release detected ($RELEASE_VERSION) — triggering beacon restart" >> "'"$LOGFILE"'"
          RESTART_SCRIPT="/home/ben/beacon/scripts/beacon-restart.sh"
          if [ -x "$RESTART_SCRIPT" ]; then
            "$RESTART_SCRIPT" "$RELEASE_VERSION" >> "'"$LOGFILE"'" 2>&1 &
            echo "[$(date -Iseconds)] beacon-restart.sh launched (PID $!)" >> "'"$LOGFILE"'"
          else
            echo "[$(date -Iseconds)] WARNING: $RESTART_SCRIPT not found or not executable — skipping restart" >> "'"$LOGFILE"'"
          fi
          ;;
      esac
      # POPANDPEEK-FORK END
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
