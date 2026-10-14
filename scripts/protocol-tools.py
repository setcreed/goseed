"""Install pinned protocol tools inside this project using the local Go toolchain."""

import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import zipfile


def main():
    root = Path(__file__).resolve().parents[1]
    lock = json.loads((root / "dependencies.lock.json").read_text())
    system = platform.system().lower()
    arch = {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64"}.get(platform.machine())
    asset = lock["protoc"]["assets"].get(f"{system}/{arch}")
    if asset is None:
        raise SystemExit("No pinned protoc archive for this host")
    temporary = root / ".tmp/protocol-tools"
    temporary.mkdir(parents=True, exist_ok=True)
    archive = temporary / asset["name"]
    version = lock["protoc"]["version"]
    url = f"https://github.com/protocolbuffers/protobuf/releases/download/v{version}/{asset['name']}"
    subprocess.run(["curl", "-fL", "--retry", "2", url, "-o", str(archive)], check=True)
    if hashlib.sha256(archive.read_bytes()).hexdigest() != asset["sha256"]:
        raise SystemExit("protoc archive SHA-256 mismatch")
    destination = root / ".tools/protoc"
    with zipfile.ZipFile(archive) as zipped:
        for name in zipped.namelist():
            if not (destination / name).resolve().is_relative_to(destination.resolve()):
                raise SystemExit("Invalid protoc archive path")
        zipped.extractall(destination)
    (destination / "bin/protoc").chmod(0o755)
    binary_directory = root / ".tools/bin"
    binary_directory.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update(GOTOOLCHAIN="local", GOWORK="off", GOBIN=str(binary_directory), GOCACHE=str(root / ".tmp/go-cache"), GOMODCACHE=str(root / ".tmp/mod-cache"))
    for module, pinned in lock["tools"].items():
        if "/protoc-gen-" in module:
            subprocess.run(["go", "install", module + "@" + pinned], cwd=root, env=env, check=True)
    print("Pinned protocol tools installed inside .tools")


if __name__ == "__main__":
    main()
