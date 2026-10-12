"""Live Python service pipeline probe, isolated from saved user profiles."""
from pathlib import Path
import multiprocessing as mp
import queue
import sys
import tempfile
import threading
import time
from unittest.mock import patch
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from desktop_service import Service

if __name__ == '__main__':
    mp.freeze_support()
    with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
        service = Service(directory)
        events = queue.Queue()
        def receive(value):
            if value.get('event') == 'state':
                data = value['data']
                events.put({'title_received': bool(data['title']),
                            'messages_received': len(data['messages']),
                            'blocked': data['blocked'], 'diagnostic': data['status']})
        service.emit = receive
        with patch('sessions.Session.save', lambda self: None):
            threading.Thread(target=service.capture_tick, daemon=True).start()
            service.dispatch('capture', {})
            result = {}
            end = time.monotonic() + 15
            try:
                while time.monotonic() < end:
                    try:
                        result = events.get(timeout=.5)
                    except queue.Empty:
                        continue
                    if result['messages_received']:
                        break
                print(result)
            finally:
                with service.lock:
                    service.stop()
