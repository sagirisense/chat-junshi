"""Opt-in capture timing trace. Never include titles, pixels or message text."""
import json
import os
import time
import threading
from pathlib import Path

_lock = threading.Lock()

def trace(stage):
    target = os.environ.get('JUNSHI_CAPTURE_TRACE')
    if not target:
        return
    try:
        with _lock, Path(target).open('a', encoding='utf-8') as file:
            file.write(json.dumps({'time': time.time(), 'pid': os.getpid(), 'stage': stage}) + '\n')
    except OSError:
        pass

