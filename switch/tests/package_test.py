"""Exercise the release packager CLI against real built NRO bytes."""
import hashlib
import json
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import tempfile
import unittest
import zipfile

ROOT=Path(__file__).resolve().parent.parent
class PackageTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.nro=ROOT.joinpath('birdroms.nro').read_bytes()
        size=struct.unpack_from('<I',cls.nro,24)[0]
        offset=struct.unpack_from('<Q',cls.nro,size+24)[0]
        cls.version_at=size+offset+0x3060
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory(prefix='birdroms-package-')
        self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)
        self.root.joinpath('scripts').mkdir()
        shutil.copyfile(ROOT/'scripts/package.py',self.root/'scripts/package.py')
        self.root.joinpath('birdroms.nro').write_bytes(self.nro)
    def run_packager(self):
        return subprocess.run([sys.executable,str(self.root/'scripts/package.py')],capture_output=True,text=True)
    def test_package_version_hash_and_token_free_layout(self):
        result=self.run_packager();self.assertEqual(result.returncode,0,result.stderr)
        with zipfile.ZipFile(self.root/'birdROMs-switch.zip') as archive:
            self.assertEqual(set(archive.namelist()),{'switch/birdroms/birdroms.nro','switch/birdroms/version.json'})
            manifest=json.loads(archive.read('switch/birdroms/version.json'))
            self.assertEqual(set(manifest),{'platform','version','nro_sha256','nro_size'})
            self.assertEqual(manifest['platform'],'switch')
            self.assertEqual(manifest['version'],self.nro[self.version_at:self.version_at+16].split(b'\0')[0].decode())
            self.assertEqual(manifest['nro_sha256'],hashlib.sha256(self.nro).hexdigest())
            self.assertEqual(manifest['nro_size'],len(self.nro))
            self.assertEqual(archive.read('switch/birdroms/birdroms.nro'),self.nro)
        before=(self.root/'birdROMs-switch.zip').read_bytes()
        self.assertEqual(self.run_packager().returncode,0)
        self.assertEqual((self.root/'birdROMs-switch.zip').read_bytes(),before)
    def test_rejects_nro_larger_than_native_budget(self):
        with (self.root/'birdroms.nro').open('r+b') as output:output.truncate(64*1024*1024+1)
        self.assertNotEqual(self.run_packager().returncode,0)
        self.assertFalse((self.root/'birdROMs-switch.zip').exists())
    def test_rejects_versions_the_native_client_cannot_parse(self):
        for version in ('dev','2019.11.01','2026.02.31','2026.11.01.x'):
            with self.subTest(version=version):
                data=bytearray(self.nro);data[self.version_at:self.version_at+16]=version.encode().ljust(16,b'\0')[:16]
                (self.root/'birdroms.nro').write_bytes(data)
                self.assertNotEqual(self.run_packager().returncode,0)
    def test_rejects_invalid_image_or_metadata(self):
        (self.root/'birdroms.nro').write_bytes(b'invalid')
        self.assertNotEqual(self.run_packager().returncode,0)
        data=bytearray(self.nro);size=struct.unpack_from('<I',data,24)[0];struct.pack_into('<Q',data,size+24,2**63)
        (self.root/'birdroms.nro').write_bytes(data)
        self.assertNotEqual(self.run_packager().returncode,0)
if __name__=='__main__':unittest.main()
