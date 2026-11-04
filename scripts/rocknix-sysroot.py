#!/usr/bin/env python3
"""Compatibility entry point for the shared Linux AArch64 link SDK."""
import pathlib
import runpy
import sys
root=pathlib.Path(__file__).resolve().parents[1]
if len(sys.argv)==1:sys.argv.append(str(root/'sysroot/rocknix'))
runpy.run_path(str(root/'scripts/linux-sysroot.py'),run_name='__main__')
