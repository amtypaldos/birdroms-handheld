#!/usr/bin/env python3
"""Run the ROCKNIX launcher against disposable app/helper fixtures."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

LAUNCHER = Path(__file__).resolve().parents[1] / "package/rocknix/birdROMs.sh"


class LauncherTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="birdroms launcher ")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.roms = self.root / "ROM storage"
        self.ports = self.roms / "ports"
        self.app = self.ports / "birdroms"
        self.app.mkdir(parents=True)
        self.script = self.ports / "birdROMs.sh"
        shutil.copyfile(LAUNCHER, self.script)
        self.bin = self.root / "commands"
        self.bin.mkdir()
        self.command("uname", "printf '%s\\n' aarch64")
        self.command("ldd", "printf '%s\\n' 'libSDL2-2.0.so.0 => /usr/lib/libSDL2-2.0.so.0'")
        self.env = os.environ.copy()
        for key in list(self.env):
            if key.startswith("BIRDROMS_") or key.startswith("SDL_"):
                self.env.pop(key)
        self.env["PATH"] = str(self.bin) + os.pathsep + self.env["PATH"]
        self.native("printf '%s\\n' \"$BIRDROMS_FIRMWARE\" \"$BIRDROMS_ROM_ROOT\" \"$PWD\"; exit 0")

    def command(self, name, body):
        path = self.bin / name
        path.write_text("#!/bin/bash\n" + body + "\n")
        path.chmod(0o755)
        return path

    def native(self, body):
        path = self.app / "birdroms"
        path.write_text("#!/bin/bash\n" + body + "\n")
        path.chmod(0o755)

    def run_launcher(self):
        return subprocess.run(["bash", str(self.script)], env=self.env,
                              cwd=self.root, text=True, capture_output=True)

    def test_default_storage_spaces_and_repeat_launch(self):
        for _ in range(2):
            result = self.run_launcher()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((self.app / "birdroms.log").read_text().splitlines(),
                             ["rocknix", str(self.roms), str(self.app)])
        self.assertFalse((self.roms / "Roms").exists())

    def test_explicit_root_preserves_exit_and_runtime_config(self):
        other = self.root / "another card"
        other.mkdir()
        self.env["BIRDROMS_ROM_ROOT"] = str(other)
        config = self.app / "config.json"
        config.write_text('{"device_token":"fixture-secret"}')
        self.native("printf '%s\\n' \"$BIRDROMS_ROM_ROOT\"; exit 37")
        result = self.run_launcher()
        self.assertEqual(result.returncode, 37, result.stderr)
        self.assertEqual((self.app / "birdroms.log").read_text().strip(), str(other))
        self.assertEqual(config.read_text(), '{"device_token":"fixture-secret"}')
        self.assertNotIn("fixture-secret", result.stdout + result.stderr)

    def test_cancel_status_is_preserved(self):
        self.native("exit 130")
        self.assertEqual(self.run_launcher().returncode, 130)

    def test_missing_app_is_reported(self):
        (self.app / "birdroms").unlink()
        result = self.run_launcher()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("binary", result.stderr.lower())

    def test_missing_storage_is_not_created(self):
        missing = self.root / "absent card"
        self.env["BIRDROMS_ROM_ROOT"] = str(missing)
        result = self.run_launcher()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("ROM", result.stderr)
        self.assertFalse(missing.exists())
        self.assertFalse((self.app / "birdroms.log").exists())

    def test_wrong_architecture_is_reported(self):
        self.command("uname", "printf '%s\\n' armv7l")
        result = self.run_launcher()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("aarch64", result.stderr)

    def test_controller_helper_is_sourced_and_called(self):
        pm = self.ports / "PortMaster"
        pm.mkdir()
        (pm / "control.txt").write_text(
            'get_controls() { export SDL_GAMECONTROLLERCONFIG="fixture-map"; }\n')
        self.native('printf "%s\\n" "$SDL_GAMECONTROLLERCONFIG"; exit 0')
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.app / "birdroms.log").read_text().strip(), "fixture-map")

    def test_invalid_helper_mapping_does_not_disable_fallback(self):
        pm = self.ports / "PortMaster"
        pm.mkdir()
        mapping = self.root / "generated.txt"
        for content in [None, "", "# only comments\n\n"]:
            with self.subTest(content=content):
                if content is not None:
                    mapping.write_text(content)
                (pm / "control.txt").write_text(
                    f'get_controls() {{ export SDL_GAMECONTROLLERCONFIG_FILE="{mapping}"; return 0; }}\n')
                self.native('test -z "${SDL_GAMECONTROLLERCONFIG_FILE:-}" || test -s "$SDL_GAMECONTROLLERCONFIG_FILE"; exit $?')
                result = self.run_launcher()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.native('printf "%s\\n" "${SDL_GAMECONTROLLERCONFIG_FILE:-builtin}"; exit 0')
                result = self.run_launcher()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertNotEqual((self.app / "birdroms.log").read_text().strip(), str(mapping))

    def test_valid_generated_mapping_is_preserved(self):
        pm = self.ports / "PortMaster"
        pm.mkdir()
        mapping = self.root / "generated.txt"
        mapping.write_text("03000000000000000000000000000000,Test,a:b0,\n")
        (pm / "control.txt").write_text(
            f'get_controls() {{ export SDL_GAMECONTROLLERCONFIG_FILE="{mapping}"; }}\n')
        self.native('printf "%s\\n" "$SDL_GAMECONTROLLERCONFIG_FILE"; exit 0')
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.app / "birdroms.log").read_text().strip(), str(mapping))

    def test_controller_helper_failure_stops_launch(self):
        pm = self.ports / "PortMaster"
        pm.mkdir()
        (pm / "control.txt").write_text('get_controls() { return 19; }\n')
        result = self.run_launcher()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("controller", result.stderr.lower())
        self.assertFalse((self.app / "birdroms.log").exists())

    def test_missing_runtime_library_is_reported(self):
        self.command("ldd", "printf '%s\\n' 'libSDL2-2.0.so.0 => not found'; exit 0")
        result = self.run_launcher()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("libSDL2", result.stderr)

    def test_no_config_or_seed_created_by_launcher(self):
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.app / "config.json").exists())
        self.assertFalse((self.app / "server.json").exists())


if __name__ == "__main__":
    unittest.main()
