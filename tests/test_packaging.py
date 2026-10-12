import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import relationship_memory


class PackagedSkillTests(unittest.TestCase):
    def test_frozen_runtime_uses_public_bundled_skill(self):
        with tempfile.TemporaryDirectory(dir=Path(__file__).parent) as directory:
            with patch.object(relationship_memory.sys, 'frozen', True, create=True), patch.object(relationship_memory.sys, '_MEIPASS', directory, create=True), patch.dict(os.environ, {'CODEX_HOME': 'unused-external-home'}):
                self.assertEqual(relationship_memory.skill_root(), Path(directory) / 'skill/goutoujunshi')

    def test_source_runtime_keeps_installed_skill(self):
        with patch.object(relationship_memory.sys, 'frozen', False, create=True), patch.dict(os.environ, {'CODEX_HOME': 'example-codex-home'}):
            self.assertEqual(relationship_memory.skill_root(), Path('example-codex-home/skills/goutoujunshi'))
