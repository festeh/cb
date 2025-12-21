#!/bin/bash
STATE_FILE="$HOME/.config/cb/screen-state.json"
mkdir -p "$(dirname "$STATE_FILE")"

# Load saved state or defaults
if [[ -f "$STATE_FILE" ]]; then
    SAVED_TAB=$(jq -r '.tab // 0' "$STATE_FILE")
    SAVED_POS=$(jq -r '.position // "right"' "$STATE_FILE")
else
    SAVED_TAB=0
    SAVED_POS="right"
fi

# Start with saved values
TAB="$SAVED_TAB"
POS="$SAVED_POS"

# Parse args (override saved values)
while [[ $# -gt 0 ]]; do
    case "$1" in
        -tab) TAB="$2"; shift 2 ;;
        -pos) POS="$2"; shift 2 ;;
        *) shift ;;
    esac
done

# Toggle check: if running and ALL params match saved state → kill and exit
if pgrep -f "cb/screen/screen" > /dev/null; then
    if [[ "$TAB" == "$SAVED_TAB" && "$POS" == "$SAVED_POS" ]]; then
        pkill -f "cb/screen/screen"
        exit 0
    fi
fi

SERVER="http://localhost:8082"

# Save state
echo "{\"tab\": $TAB, \"position\": \"$POS\"}" > "$STATE_FILE"

# Check if flow has content, trigger if empty
HAS_CONTENT=$(curl -s "$SERVER/flow/has-content" | grep -o '"has_content":true')

if [[ -z "$HAS_CONTENT" ]]; then
    # Trigger flow in background
    curl -s -X POST "$SERVER/flow/trigger" > /dev/null &

    # Wait for first response (max 20 attempts, 5 sec each)
    ATTEMPTS=0
    while [[ -z "$HAS_CONTENT" ]]; do
        sleep 5
        HAS_CONTENT=$(curl -s "$SERVER/flow/has-content" | grep -o '"has_content":true')
        ATTEMPTS=$((ATTEMPTS + 1))
        if [[ $ATTEMPTS -ge 20 ]]; then
            break
        fi
    done
fi

# Launch screen
pkill -f "cb/screen/screen"
exec /home/dima/projects/cb/screen/screen -server "$SERVER" -tab "$TAB"
