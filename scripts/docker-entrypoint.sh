#!/bin/bash
set -e
if [ $# -eq 0 ]; then
	exec tmux -2 new-session
else
	exec tmux -2 new-session "$@"
fi
