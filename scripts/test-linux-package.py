#!/usr/bin/env python3
"""Exercise the actual Linux ZIP packager CLI, without a cross compiler."""
import hashlib
import json
import pathlib
import struct
import subprocess
import tempfile
import unittest
import zipfile

ROOT = pathlib.Path(__file__).resolve().parents[1]

class PackageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.dir = pathlib.Path(self.temp.name)
        self.binary = self.dir / 'birdroms'
        # ELF64 little-endian ET_EXEC/AArch64 header; no executable code.
        header = bytearray(128)
        header[:7] = b'\x7fELF\x02\x01\x01'
        struct.pack_into('<HHI', header, 16, 2, 183, 1)
        struct.pack_into('<H', header, 52, 64)
        self.binary.write_bytes(header)
        self.output = self.dir / 'out.zip'

    def package(self, version='2026.11.03-alpha.1', target='rocknix', server='https://roms.example.com'):
        return subprocess.run(['python3', str(ROOT/'scripts/package-linux.py'), target, version,
                               '--binary',str(self.binary),'--output',str(self.output),'--server-url',server],
                              capture_output=True,text=True)

    def test_layout_manifest_and_token_free_seed(self):
        p=self.package();self.assertEqual(p.returncode,0,p.stderr)
        with zipfile.ZipFile(self.output) as z:
            names=set(z.namelist())
            self.assertIn('birdROMs.sh',names)
            self.assertIn('birdroms/birdroms',names)
            self.assertIn('birdroms/assets/fonts/OFL.txt',names)
            self.assertFalse(any(n.endswith('config.json') for n in names))
            self.assertFalse(any(n.startswith('/') or '..' in pathlib.PurePosixPath(n).parts for n in names))
            manifest=json.loads(z.read('birdroms/version.json'))
            self.assertEqual(manifest,dict(platform='rocknix',arch='aarch64',version='2026.11.03-alpha.1',layout='ports-v1',binary_size=128,binary_sha256=hashlib.sha256(self.binary.read_bytes()).hexdigest()))
            self.assertEqual(json.loads(z.read('birdroms/server.json')),{'server_url':'https://roms.example.com'})
            for name in ['birdROMs.sh','birdroms/birdroms']:
                self.assertTrue((z.getinfo(name).external_attr>>16)&0o111)
        sha=self.output.with_suffix('.zip.sha256').read_text().split()[0]
        self.assertEqual(sha,hashlib.sha256(self.output.read_bytes()).hexdigest())

    def test_batocera_has_own_manifest_launcher_and_docs(self):
        p=self.package(target='batocera',version='2026.11.04-alpha.1')
        self.assertEqual(p.returncode,0,p.stderr)
        with zipfile.ZipFile(self.output) as z:
            manifest=json.loads(z.read('birdroms/version.json'))
            self.assertEqual(manifest['platform'],'batocera')
            self.assertEqual(manifest['version'],'2026.11.04-alpha.1')
            launcher=z.read('birdROMs.sh').decode()
            self.assertIn('BIRDROMS_FIRMWARE=batocera',launcher)
            self.assertNotIn('/PortMaster/',launcher)
            self.assertIn('Batocera',z.read('birdroms/README.md').decode())
            self.assertFalse(any(n.endswith('config.json') for n in z.namelist()))

    def test_reproducible_archive(self):
        self.assertEqual(self.package().returncode,0)
        before=self.output.read_bytes()
        self.assertEqual(self.package().returncode,0)
        self.assertEqual(before,self.output.read_bytes())

    def test_wrong_architecture_rejected(self):
        raw=bytearray(self.binary.read_bytes());struct.pack_into('<H',raw,18,62);self.binary.write_bytes(raw)
        self.assertNotEqual(self.package().returncode,0)
        self.assertFalse(self.output.exists())

    def test_invalid_version_target_and_url_rejected(self):
        for args in [dict(version='2026.02.30-alpha.1'),dict(target='knulli'),dict(server='https://user:secret@example.com'),dict(server='file:///tmp'),dict(server='https://example.com/#token')]:
            p=self.package(**args);self.assertNotEqual(p.returncode,0,p.stdout)
            self.assertFalse(self.output.exists())

if __name__=='__main__':unittest.main()
