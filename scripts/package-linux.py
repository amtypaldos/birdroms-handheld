#!/usr/bin/env python3
"""Build deterministic, token-free Linux firmware Ports archives."""
import argparse
import datetime
import hashlib
import json
import pathlib
import re
import struct
import urllib.parse
import zipfile

ROOT=pathlib.Path(__file__).resolve().parents[1]

def main():
    ap=argparse.ArgumentParser()
    ap.add_argument('target',choices=['rocknix','batocera']);ap.add_argument('version')
    ap.add_argument('--binary',type=pathlib.Path);ap.add_argument('--output',type=pathlib.Path)
    ap.add_argument('--server-url',default='')
    args=ap.parse_args()
    if not re.fullmatch(r'\d{4}\.\d{2}\.\d{2}(?:\.\d+)?(?:-alpha\.[1-9]\d*)?',args.version):ap.error('invalid date version')
    try:datetime.date(*map(int,args.version.split('-')[0].split('.')[:3]))
    except ValueError:ap.error('invalid calendar date')
    url=args.server_url.rstrip('/')
    if url:
        parsed=urllib.parse.urlsplit(url)
        if parsed.scheme not in ('https','http') or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment or any(ord(c)<=32 for c in url):ap.error('server URL must be HTTP(S) without credentials/query/fragment')
    binary=args.binary or ROOT/f'dist/{args.target}/birdroms'
    raw=binary.read_bytes()
    if len(raw)<64 or raw[:7]!=b'\x7fELF\x02\x01\x01' or struct.unpack_from('<H',raw,18)[0]!=183 or struct.unpack_from('<H',raw,16)[0] not in (2,3):ap.error('binary must be a Linux AArch64 ELF64 executable')
    if len(raw)>64*1024*1024:ap.error('binary exceeds 64 MiB')
    manifest=dict(platform=args.target,arch='aarch64',version=args.version,layout='ports-v1',binary_sha256=hashlib.sha256(raw).hexdigest(),binary_size=len(raw))
    files={'birdROMs.sh':((ROOT/f'package/{args.target}/birdROMs.sh').read_bytes(),0o755),'birdroms/birdroms':(raw,0o755)}
    for source,name in [('assets/fonts/ChakraPetch-Bold.ttf','assets/fonts/ChakraPetch-Bold.ttf'),('assets/fonts/OFL.txt','assets/fonts/OFL.txt'),('LICENSE','LICENSE'),('THIRD_PARTY_NOTICES.txt','THIRD_PARTY_NOTICES.txt'),(f'docs/platforms/{args.target}.md','README.md')]:
        files['birdroms/'+name]=((ROOT/source).read_bytes(),0o644)
    for name,data in [('version.json',manifest),('server.json',{'server_url':url})]:files['birdroms/'+name]=((json.dumps(data,sort_keys=True,indent=2)+'\n').encode(),0o644)
    output=args.output or ROOT/f'dist/birdROMs-{args.target}-aarch64.zip';output.parent.mkdir(parents=True,exist_ok=True)
    temp=output.with_suffix(output.suffix+'.tmp')
    try:
        with zipfile.ZipFile(temp,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
            for name,(data,mode) in sorted(files.items()):
                info=zipfile.ZipInfo(name,(2020,1,1,0,0,0));info.create_system=3;info.external_attr=(0o100000|mode)<<16;info.compress_type=zipfile.ZIP_DEFLATED;z.writestr(info,data)
        temp.replace(output)
    finally:temp.unlink(missing_ok=True)
    sha=hashlib.sha256(output.read_bytes()).hexdigest()
    output.with_suffix(output.suffix+'.sha256').write_text(sha+'  '+output.name+'\n')
    print(output,output.stat().st_size,sha)

if __name__=='__main__':main()
