import unittest
import numpy as np
from types import SimpleNamespace
from unittest.mock import patch
from app.capture import Capture


class CaptureTests(unittest.TestCase):
    def test_latest_frame_available_even_when_message_area_not_yet_recognized(self):
        with patch('windows_capture.WindowsCapture'), patch('app.capture.chat_area', return_value=None):
            capture = Capture(123)
            capture.on_frame_arrived(SimpleNamespace(frame_buffer=np.full((80, 100, 4), 238, np.uint8)), None)
            self.assertIsNone(capture.settled())
            self.assertEqual(capture.snapshot().shape, (80, 100, 3))

    def test_installed_capture_api_accepts_target_argument(self):
        import inspect
        from windows_capture import WindowsCapture
        self.assertIn('window_hwnd', inspect.signature(WindowsCapture).parameters)

    def test_capture_initializes_with_exact_handle(self):
        with patch('windows_capture.WindowsCapture') as api:
            capture = Capture(123)
            api.assert_called_once_with(cursor_capture=None, draw_border=None, window_hwnd=123)
            api.return_value.start_free_threaded.assert_called_once()
            capture.stop()
