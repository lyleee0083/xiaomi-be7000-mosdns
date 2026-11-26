#!/usr/bin/env python3
import argparse
import logging
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

logger = logging.getLogger(__name__)


def build_be7000(root: Path, version: str) -> Path:
    zig = shutil.which("zig")
    if zig is None:
        raise SystemExit("Zig is required on PATH to build for Linux ARM64 musl.")
    zig = Path(zig).resolve()
    licenses = {
        "LICENSES/zig.txt": zig.parent / "LICENSE",
        "LICENSES/musl.txt": zig.parent / "lib" / "libc" / "musl" / "COPYRIGHT",
    }
    for notice in licenses.values():
        if not notice.is_file():
            raise SystemExit(f"Missing {notice}. Use an official Zig distribution.")

    release_dir = root / "release"
    release_dir.mkdir(exist_ok=True)
    archive_name = f"mosdns-xiaomi-be7000-{version}.zip"
    env = {
        **os.environ,
        "GOOS": "linux",
        "GOARCH": "arm64",
        "GOARM64": "v8.0",
        "CGO_ENABLED": "1",
        "CC": f'"{zig}" cc -target aarch64-linux-musl',
    }

    logger.info("building %s for linux-arm64-musl", archive_name)
    with tempfile.TemporaryDirectory(prefix="be7000-", dir=release_dir) as temp:
        temp_dir = Path(temp)
        binary = temp_dir / "mosdns"
        subprocess.run(
            ["go", "build", "-mod=readonly", "-trimpath", "-buildmode=exe",
             "-tags", "netgo,osusergo", "-ldflags",
             "-s -w -linkmode=external -extldflags=-static",
             "-o", str(binary), "."],
            cwd=root, env=env, stdout=sys.stderr, check=True,
        )
        archive = temp_dir / archive_name
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED, compresslevel=5) as package:
            entry = zipfile.ZipInfo.from_file(binary, "mosdns")
            entry.create_system = 3
            entry.external_attr = 0o100755 << 16
            package.writestr(entry, binary.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=5)
            package.write(root / "README.md", "README.md")
            package.write(root / "LICENSE", "LICENSE")
            for name, notice in licenses.items():
                package.write(notice, name)
        return archive.replace(release_dir / archive_name)


def main():
    parser = argparse.ArgumentParser(description="Build the Xiaomi BE7000 MosDNS archive.")
    parser.add_argument("--tag", help="Require the GitHub release tag to match VERSION.")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent
    version = (root / "VERSION").read_text(encoding="ascii").strip()
    if re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version) is None:
        parser.error("VERSION must have the form x.y.z.")
    if args.tag is not None and args.tag != f"v{version}":
        parser.error(f"expected tag v{version}, got {args.tag}")
    logging.basicConfig(level=logging.INFO)
    print(build_be7000(root, version).relative_to(root).as_posix())


if __name__ == "__main__":
    main()
