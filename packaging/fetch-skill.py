"""Fetch pinned public Skill files only; never copy the user's installed Skill."""
import io
import json
from pathlib import Path
from urllib.request import urlopen
from zipfile import ZipFile

root = Path(__file__).resolve().parent
source = json.loads((root / 'skill-source.json').read_text(encoding='utf-8'))
with urlopen('https://codeload.github.com/shengjidaguai-china/goutoujunshi/zip/' + source['commit'], timeout=30) as response:
    archive = response.read()
with ZipFile(io.BytesIO(archive)) as bundle:
    for name in bundle.namelist():
        relative = Path(*Path(name).parts[1:])
        if not relative.parts or '..' in relative.parts: continue
        allowed = relative.as_posix() in ('SKILL.md', 'LICENSE', 'LICENSE.zh-CN.md', 'scripts/memory_store.py') or (relative.parts[0] == 'references' and relative.suffix == '.md')
        if allowed:
            target = root / 'vendor/goutoujunshi' / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(bundle.read(name))
print('Pinned public Skill downloaded')
