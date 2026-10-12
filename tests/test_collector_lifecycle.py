import tempfile
import threading
import unittest
from unittest.mock import patch

from desktop_service import Service


class CollectorLifecycleTests(unittest.TestCase):
    def test_background_thread_prewarm_pause_resume_and_shutdown(self):
        prepared = threading.Event()
        capturing = threading.Event()
        finished = threading.Event()

        def worker(output, enabled, prewarm, shutdown):
            self.assertTrue(prewarm)
            prepared.set()
            enabled.wait()
            capturing.set()
            shutdown.wait()
            finished.set()

        with tempfile.TemporaryDirectory() as directory, patch('desktop_service.RelationshipMemory') as memory, patch('desktop_service.run', side_effect=worker):
            memory.return_value.rows.return_value = []
            memory.return_value.status.return_value = {'consent_enabled': False}
            service = Service(directory)
            service.emit = lambda value: None
            try:
                service.ensure_collector()
                self.assertTrue(prepared.wait(1))
                self.assertIsInstance(service.child, threading.Thread)
                self.assertFalse(capturing.is_set())
                original = service.child
                service.dispatch('capture', {})
                self.assertTrue(capturing.wait(1))
                service.dispatch('capture', {})
                self.assertFalse(service.enabled.is_set())
                service.dispatch('capture', {})
                self.assertIs(service.child, original)
            finally:
                service.stop()
            self.assertTrue(finished.is_set())
            self.assertFalse(original.is_alive())
            self.assertFalse(service.enabled.is_set())

