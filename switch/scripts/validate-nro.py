#!/usr/bin/env python3
"""Validate NRO layout including embedded NACP/icon/RomFS assets (not hardware execution)."""
import pathlib, struct
p = pathlib.Path(__file__).resolve().parent.parent / 'birdroms.nro'
b = p.read_bytes()
assert b[0x10:0x14] == b'NRO0', 'Missing NRO magic'
size = struct.unpack_from('<I', b, 0x18)[0]
assert 0x80 <= size <= len(b), 'Invalid NRO image length'
assert b[size:size + 4] == b'ASET', 'Missing asset header'
for i, name in enumerate(('icon', 'NACP', 'RomFS')):
    offset, length = struct.unpack_from('<QQ', b, size + 8 + i * 16)
    assert length > 0 and size + offset + length <= len(b), f'Invalid {name}'
print(f'{p.name}: valid NRO layout with icon, NACP, RomFS; {len(b):,} bytes')
