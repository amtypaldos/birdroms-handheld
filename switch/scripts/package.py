#!/usr/bin/env python3
"""Build the token-free Switch archive consumed by birdROMs publish_handheld."""
import datetime
import hashlib
import json
from pathlib import Path
import re
import struct
import zipfile

root = Path(__file__).resolve().parent.parent
nro = root / 'birdroms.nro'
if not nro.is_file() or not 128 <= nro.stat().st_size <= 64 * 1024 * 1024:
    raise SystemExit('NRO is missing or exceeds the native 64 MiB budget')
data = nro.read_bytes()
if len(data) < 128 or data[16:20] != b'NRO0':
    raise SystemExit('birdroms.nro is missing or invalid; build it first')
image_size = struct.unpack_from('<I', data, 24)[0]
if data[image_size:image_size + 4] != b'ASET':
    raise SystemExit('NRO metadata is missing')
nacp_offset, nacp_size = struct.unpack_from('<QQ', data, image_size + 24)
if nacp_size < 0x3070 or image_size + nacp_offset + nacp_size > len(data):
    raise SystemExit('Invalid NACP metadata bounds')
version = data[image_size + nacp_offset + 0x3060:image_size + nacp_offset + 0x3070].split(b'\0')[0].decode('ascii')
match = re.fullmatch(r'(\d{4})\.(\d{2})\.(\d{2})(?:\.(\d+))?', version)
if not match or int(match[1]) < 2020 or (match[4] and int(match[4]) > 0xffffffff):
    raise SystemExit('Build a date-versioned release before packaging')
datetime.date(*map(int, match.group(1, 2, 3)))
manifest = {'platform': 'switch', 'version': version,
            'nro_sha256': hashlib.sha256(data).hexdigest(), 'nro_size': len(data)}
output = root / 'birdROMs-switch.zip'
partial = output.with_suffix('.zip.part')
with zipfile.ZipFile(partial, 'w', compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
    for name, contents in [('birdroms.nro', data), ('version.json', (json.dumps(manifest, indent=2) + '\n').encode())]:
        entry = zipfile.ZipInfo('switch/birdroms/' + name, (1980, 1, 1, 0, 0, 0))
        entry.compress_type = zipfile.ZIP_DEFLATED
        entry.external_attr = 0o100644 << 16
        archive.writestr(entry, contents, compresslevel=9)
if partial.stat().st_size > 64 * 1024 * 1024 - 4096:
    partial.unlink()
    raise SystemExit('Switch archive exceeds the supported update size')
partial.replace(output)
(root / 'birdroms.nro.sha256').write_text(f'{manifest["nro_sha256"]}  birdroms.nro\n')
(root / 'birdROMs-switch.zip.sha256').write_text(f'{hashlib.sha256(output.read_bytes()).hexdigest()}  birdROMs-switch.zip\n')
print(f'{output.name}: version {version}; {output.stat().st_size:,} bytes; no runtime config or token')
