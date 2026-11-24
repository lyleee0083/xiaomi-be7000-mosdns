#!/usr/bin/env python3
import argparse
import logging
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile

ARCHIVE_NAME = "mosdns-xiaomi-be7000-linux-arm64.zip"
logger = logging.getLogger(__name__)


def build_be7000():
    root = Path(__file__).resolve().parent
    release_dir = root / "release"
    release_dir.mkdir(exist_ok=True)
    version = subprocess.check_output(
        ["git", "describe", "--tags", "--long", "--always", "--dirty"],
        cwd=root, encoding="utf-8",
    ).strip()
    env = {
        **os.environ,
        "GOOS": "linux",
        "GOARCH": "arm64",
        "GOARM64": "v8.0",
        "CGO_ENABLED": "0",
    }

    logger.info("building %s", ARCHIVE_NAME)
    with tempfile.TemporaryDirectory(prefix="be7000-", dir=release_dir) as temp:
        temp_dir = Path(temp)
        binary = temp_dir / "mosdns"
        subprocess.run(
            ["go", "build", "-mod=readonly", "-trimpath", "-buildmode=exe",
             "-ldflags", f"-s -w -X main.version={version}", "-o", str(binary), "."],
            cwd=root, env=env, check=True,
        )
        archive = temp_dir / ARCHIVE_NAME
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED, compresslevel=5) as package:
            entry = zipfile.ZipInfo.from_file(binary, "mosdns")
            entry.create_system = 3
            entry.external_attr = 0o100755 << 16
            package.writestr(entry, binary.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=5)
            package.write(root / "README.md", "README.md")
            package.write(root / "LICENSE", "LICENSE")
        archive.replace(release_dir / ARCHIVE_NAME)


if __name__ == "__main__":
    argparse.ArgumentParser(description="Build the Xiaomi BE7000 MosDNS archive.").parse_args()
    logging.basicConfig(level=logging.INFO)
    build_be7000()
