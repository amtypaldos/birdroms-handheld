#!/usr/bin/env python3
"""Pinned link-only sysroot. SDL shared libraries are NOT bundled at runtime."""
import hashlib
import json
import pathlib
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

ROOT=pathlib.Path(__file__).resolve().parents[1]
HEADERS=[
 ('SDL2','2.28.5','332cb37d0be20cb9541739c61f79bae5a477427d79ae85e352089afdaf6666e4','https://github.com/libsdl-org/SDL/releases/download'),
 ('SDL2_ttf','2.20.2','9dc71ed93487521b107a2c4a9ca6bf43fb62f6bddd5c26b055e6b91418a22053','https://github.com/libsdl-org/SDL_ttf/releases/download')]

def fetch(url,path,sha):
    with urllib.request.urlopen(url,timeout=120) as r,path.open('wb') as f:shutil.copyfileobj(r,f)
    if hashlib.sha256(path.read_bytes()).hexdigest()!=sha:raise ValueError('dependency SHA-256 mismatch: '+url)

def main():
    dest=pathlib.Path(sys.argv[1] if len(sys.argv)>1 else ROOT/'sysroot/rocknix').resolve()
    (dest/'include/SDL2').mkdir(parents=True,exist_ok=True);(dest/'lib').mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory() as temp:
        work=pathlib.Path(temp)
        for name,version,sha,base in HEADERS:
            archive=work/(name+'.tar.gz');fetch(f'{base}/release-{version}/{name}-{version}.tar.gz',archive,sha)
            # Extract only header files as flat files; no archive path is followed.
            prefix=f'{name}-{version}/'+('include/' if name=='SDL2' else '')
            with tarfile.open(archive) as tar:
                for member in tar:
                    if member.isfile() and member.name.startswith(prefix) and member.name.endswith('.h'):
                        relative=member.name[len(prefix):]
                        if '/' not in relative:
                            with tar.extractfile(member) as src,(dest/'include/SDL2'/relative).open('wb') as out:shutil.copyfileobj(src,out)
        lock=json.loads((ROOT/'scripts/locks/rocknix-link-libraries.json').read_text())
        for item in lock:
            deb=work/(item['name']+'.deb');fetch(item['url'],deb,item['sha256'])
            unpack=work/item['name'];subprocess.run(['dpkg-deb','-x',str(deb),str(unpack)],check=True)
            for lib in (unpack/'usr/lib/aarch64-linux-gnu').glob('*.so*'):
                target=dest/'lib'/lib.name
                if target.exists() or target.is_symlink():target.unlink()
                if lib.is_symlink():target.symlink_to(lib.readlink())
                else:shutil.copyfile(lib,target)
        for name,pattern in [('libSDL2.so','libSDL2-2.0.so.0.*'),('libSDL2_ttf.so','libSDL2_ttf-2.0.so.0.*')]:
            target=dest/'lib'/name
            if target.exists() or target.is_symlink():target.unlink()
            target.symlink_to(sorted((dest/'lib').glob(pattern))[0].name)
    print('Pinned ROCKNIX link sysroot:',dest)

if __name__=='__main__':main()
