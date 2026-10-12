"""Manual live capture diagnosis; only prints counts, never message text."""
import multiprocessing as mp
from pathlib import Path
import queue
import sys
import time
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from collector import run

if __name__ == '__main__':
    mp.freeze_support()
    enabled = mp.Event()
    output = mp.Queue()
    enabled.set()
    child = mp.Process(target=run, args=(output, enabled), daemon=True)
    child.start()
    end = time.monotonic() + 15
    counts = {'title_recognized': False, 'messages_received': 0}
    try:
        while time.monotonic() < end:
            try:
                item = output.get(timeout=.5)
            except queue.Empty:
                continue
            if item[0] == 'chat':
                counts['title_recognized'] = True
            elif item[0] == 'lines':
                counts['messages_received'] += len(item[2])
                break
            elif item[0] in ('dead', 'blocked', 'status'):
                counts['diagnostic'] = item[1]
        print(counts)
    finally:
        enabled.clear()
        child.terminate()
        child.join(timeout=3)
