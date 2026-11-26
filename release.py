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

logger = logging.getLogger(__name__)


def build_be7000(root: Path, version: str) -> Path:
    zig = shutil.which("zig")
    if zig is None:
        raise SystemExit("Zig is required on PATH to build for Linux ARM64 musl.")
    zig = Path(zig).resolve()
    release_dir = root / "release"
    release_dir.mkdir(exist_ok=True)
    asset = release_dir / f"mosdns-xiaomi-be7000-{version}"
    env = {
        **os.environ,
        "GOOS": "linux",
        "GOARCH": "arm64",
        "GOARM64": "v8.0",
        "CGO_ENABLED": "1",
        "CC": f'"{zig}" cc -target aarch64-linux-musl',
    }

    logger.info("building %s for linux-arm64-musl", asset.name)
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
        return binary.replace(asset)


def main():
    parser = argparse.ArgumentParser(description="Build the Xiaomi BE7000 MosDNS executable.")
    parser.parse_args()
    root = Path(__file__).resolve().parent
    version = (root / "VERSION").read_text(encoding="ascii").strip()
    if re.fullmatch(r"(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*)){2}", version) is None:
        parser.error("VERSION must have the form x.y.z.")
    logging.basicConfig(level=logging.INFO)
    print(build_be7000(root, version).relative_to(root).as_posix())


if __name__ == "__main__":
    main()
