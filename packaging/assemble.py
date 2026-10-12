"""Assemble only build outputs and public licenses; exclude all user data."""
import hashlib
import importlib.metadata
import json
import shutil
import sys
from pathlib import Path

root = Path(__file__).resolve().parents[1]
dist = root / 'packaging/dist'
target = dist / 'ChatJunshi'
if target.exists():
    if target.resolve().parent != dist.resolve() or target.is_symlink(): raise RuntimeError('Unsafe assembly path')
    shutil.rmtree(target)
target.mkdir(parents=True)
shutil.copyfile(root / 'desktop/build/bin/chat-junshi-installer.exe', target / 'chat-junshi.exe')
shutil.copytree(dist / 'service/junshi-service', target / 'service')
for name in ('LICENSE', 'NOTICE'):
    shutil.copyfile(root / name, target / name)
shutil.copytree(root / 'licenses', target / 'licenses')
shutil.copyfile(root / 'packaging/vendor/goutoujunshi/LICENSE', target / 'licenses/goutoujunshi-LICENSE')
shutil.copyfile(root / 'packaging/skill-source.json', target / 'skill-source.json')
for package in importlib.metadata.distributions():
    for name in package.files or []:
        if any('license' in part.lower() or 'copying' in part.lower() or part.lower() in ('notice','notice.txt') for part in name.parts):
            file = Path(package.locate_file(name))
            if not file.is_file(): continue
            safe_name = '__'.join(part for part in name.parts if part not in ('..', '.'))
            destination = target / 'licenses/dependencies' / package.metadata['Name'] / safe_name
            if not destination.resolve().is_relative_to(target.resolve()): raise RuntimeError('Unsafe license path')
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(file, destination)
python_license = Path(sys.base_prefix) / 'LICENSE.txt'
if python_license.exists(): shutil.copyfile(python_license, target / 'licenses/Python-LICENSE.txt')
(target / 'INSTALL-README.txt').write_text('聊天军师 0.1.0\n内置 Python、OCR 和公开 MIT 狗头军师 Skill。\n用户数据：%LOCALAPPDATA%\\ChatJunshi\\data\n卸载默认保留用户数据，删除请在应用内操作或自行备份后处理。\n自定义 API 需填写自己的密钥；Codex 模式需另行安装并登录 Codex CLI。\n需要 Windows 10 1903+ x64 / Windows 11 与 WebView2 Runtime。\n本安装包未做代码签名，系统可能提示未知发布者。\n来源与许可证见 NOTICE、licenses/，源码：https://github.com/sagirisense/chat-junshi\n', encoding='utf-8')
manifest = []
for file in sorted(target.rglob('*')):
    if not file.is_file(): continue
    relative = file.relative_to(target).as_posix()
    if file.suffix in ('.db','.sqlite','.sqlite3','.log') or file.name in ('auth.json','.env','selected-session.json') or (relative.startswith('data/') or ('/data/' in '/' + relative and not relative.startswith('service/_internal/cv2/data/'))):
        raise RuntimeError('Private runtime file found in payload')
    manifest.append({'path':relative,'size':file.stat().st_size,'sha256':hashlib.sha256(file.read_bytes()).hexdigest()})
(target / 'payload-manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf-8')
print('Clean installer payload files:',len(manifest))
