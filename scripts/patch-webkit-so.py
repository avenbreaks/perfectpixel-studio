#!/usr/bin/env python3
"""Patch the WebKitGTK auxiliary-process paths baked into a bundled
libwebkit2gtk shared library, so a Wails AppImage can find its OWN
WebKitNetworkProcess / WebKitWebProcess / injected-bundle on hosts that do not
ship the matching webkit2gtk ABI (e.g. a 4.0 AppImage running on Ubuntu 24.04,
which only ships 4.1).

Why this is needed
------------------
WebKitGTK release builds ignore WEBKIT_EXEC_PATH and WEBKIT_INJECTED_BUNDLE_PATH
(both are wrapped in ``#if ENABLE(DEVELOPER_MODE)``, and Ubuntu builds with
developer mode OFF -- verified: ``strings libwebkit2gtk-4.1.so.0`` contains no
"WEBKIT_EXEC_PATH"). Instead the helper executables are spawned at the
compile-time PKGLIBEXECDIR:

    /usr/lib/x86_64-linux-gnu/webkit2gtk-4.0                      (40 bytes)
    /usr/lib/x86_64-linux-gnu/webkit2gtk-4.0/injected-bundle/     (57 bytes)

An AppImage cannot write to the host ``/usr``, so both strings are rewritten
in-place to a fixed, writable directory (default ``/tmp/pp-webkit4.0``) that the
AppRun populates with the bundled helpers before exec.

The rewrite is strictly length-preserving: replacements are null-padded to the
original byte length, so the ELF layout, file size, SONAME and relocations are
untouched.

Usage
-----
    patch-webkit-so.py --abi 4.0 --target /tmp/pp-webkit4.0 <libwebkit2gtk.so> ...

The longer injected-bundle string is patched first (it contains the shorter
PKGLIBEXECDIR string as a prefix).
"""

import argparse
import sys

ABIS = ("4.0", "4.1")


def _pad(data: bytes, length: int) -> bytes:
    if len(data) > length:
        raise ValueError(
            f"replacement {data!r} is {len(data)} bytes, longer than the "
            f"{length}-byte original"
        )
    return data + b"\0" * (length - len(data))


def patch_blob(data: bytes, abi: str, target: str) -> tuple[bytes, int, int]:
    exec_old = f"/usr/lib/x86_64-linux-gnu/webkit2gtk-{abi}".encode()
    injected_old = f"/usr/lib/x86_64-linux-gnu/webkit2gtk-{abi}/injected-bundle/".encode()

    target = target.rstrip("/")
    exec_new = _pad(target.encode(), len(exec_old))
    injected_new = _pad((target + "/injected-bundle/").encode(), len(injected_old))

    # Patch the longer injected-bundle string first: it contains exec_old as a
    # prefix, so a naive exec_old -> exec_new replace would corrupt it.
    injected_count = data.count(injected_old)
    data = data.replace(injected_old, injected_new)

    exec_count = data.count(exec_old)
    data = data.replace(exec_old, exec_new)

    return data, exec_count, injected_count


def patch_file(path: str, abi: str, target: str) -> tuple[int, int]:
    with open(path, "rb") as fh:
        data = fh.read()
    patched, exec_count, injected_count = patch_blob(data, abi, target)
    if exec_count == 0 and injected_count == 0:
        raise SystemExit(
            f"{path}: no webkit2gtk-{abi} path strings found (wrong ABI or "
            "already patched)"
        )
    if len(patched) != len(data):
        raise SystemExit(f"{path}: size changed during patch (aborting)")
    with open(path, "wb") as fh:
        fh.write(patched)
    return exec_count, injected_count


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--abi", choices=ABIS, default="4.0")
    parser.add_argument("--target", default="/tmp/pp-webkit4.0")
    parser.add_argument("libs", nargs="+", metavar="LIB")
    args = parser.parse_args()

    for lib in args.libs:
        exec_count, injected_count = patch_file(lib, args.abi, args.target)
        print(
            f"patched {lib}: PKGLIBEXECDIR x{exec_count}, "
            f"injected-bundle x{injected_count} -> {args.target}"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())