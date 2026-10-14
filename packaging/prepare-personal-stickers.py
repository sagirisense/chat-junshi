"""Prepare downloaded personal images; never included in installer assets."""
import hashlib
import json
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

root = Path(__file__).resolve().parents[1] / 'data' / 'stickers'
sources = json.loads((root / 'sources.json').read_text(encoding='utf-8-sig'))
rows = []
seen = set()
existing = {row['id']: row for row in json.loads((root / 'index.json').read_text(encoding='utf-8'))} if (root / 'index.json').exists() else {}
sheet = Image.new('RGB', (960, ((len(sources)+5)//6)*180), 'white')
font = ImageFont.truetype('C:/Windows/Fonts/msyh.ttc', 14)
draw = ImageDraw.Draw(sheet)
for i, source in enumerate(sources):
    with Image.open(root / 'originals' / source['file']) as gif:
        gif.seek(getattr(gif, 'n_frames', 1) // 3)
        frame = gif.convert('RGBA')
    temporary = root / f'frame-{i:02}.png'
    frame.save(temporary)
    digest = hashlib.sha256(temporary.read_bytes()).hexdigest()
    temporary.replace(root / f'{digest}.png')
    row = dict(id=digest, name=f"{source['series']}-{i+1:02}", tags='', suitable='', avoid='', source=source['source'], license=source['license'], intimate=False, enabled=False)
    if digest not in seen:
        rows.append(existing.get(digest, row))
        seen.add(digest)
    frame.thumbnail((140, 140))
    x, y = (i%6)*160, (i//6)*180
    sheet.paste(frame, (x+(160-frame.width)//2, y), frame)
    draw.text((x+6, y+143), f"{i+1:02} {source['series']}", font=font, fill='black')
(root / 'index.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2), encoding='utf-8')
sheet.save(root / 'contact-sheet.png')
print(f'Prepared {len(rows)} personal stickers at {root}')
