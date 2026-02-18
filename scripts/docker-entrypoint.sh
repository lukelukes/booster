#!/bin/bash
set -e

SESSION="booster"

if [ -t 0 ]; then
	exec tmux -2 new-session -s "$SESSION" "$@"
else
	tmux -2 new-session -d -s "$SESSION" "$@"
	exec tail -f /dev/null
fi
