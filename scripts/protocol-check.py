"""Regenerate the protocol proof into a disposable directory and compare."""

import json
import os
from pathlib import Path
import subprocess
import tempfile


def main():
    root = Path(__file__).resolve().parents[1]
    lock = json.loads((root / "dependencies.lock.json").read_text())
    protoc = root / ".tools/protoc/bin/protoc"
    tools = root / ".tools/bin"
    if not protoc.is_file():
        raise SystemExit("Run make protocol-tools first")
    result = subprocess.run([str(protoc), "--version"], check=True, text=True, capture_output=True)
    if result.stdout.strip() != "libprotoc " + lock["protoc"]["version"]:
        raise SystemExit("protoc version mismatch")
    for module, pinned in lock["tools"].items():
        if "/protoc-gen-" not in module:
            continue
        result = subprocess.run(["go", "version", "-m", str(tools / module.rsplit("/", 1)[1])], check=True, text=True, capture_output=True)
        if module.endswith("/protoc-gen-go-grpc"):
            expected = module  # This plugin has a separately versioned module.
        elif "/cmd/" in module:
            expected = module.split("/cmd/")[0]
        else:
            expected = module.rsplit("/", 1)[0]
        if not any(line.split()[:3] == ["mod", expected, pinned] for line in result.stdout.splitlines()):
            raise SystemExit("Protocol plugin version mismatch: " + module)
    fixture = root / "tests/compatibility/protocol"
    temporary = root / ".tmp/protocol-check"
    temporary.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["PATH"] = str(tools) + os.pathsep + env.get("PATH", "")
    with tempfile.TemporaryDirectory(dir=temporary) as directory:
        arguments = [str(protoc), "-I", str(fixture),
            "--go_out=paths=source_relative:" + directory,
            "--go-grpc_out=paths=source_relative:" + directory,
            "--grpc-gateway_out=paths=source_relative,generate_unbound_methods=true:" + directory,
            "--openapiv2_out=generate_unbound_methods=true:" + directory, str(fixture / "smoke.proto")]
        subprocess.run(arguments, cwd=root, env=env, check=True)
        for generated in Path(directory).iterdir():
            target = fixture / generated.name
            if not target.is_file() or target.read_bytes() != generated.read_bytes():
                raise SystemExit("Generated protocol file differs: " + generated.name)
    print("Pinned protoc, plugins and generated files match")


if __name__ == "__main__":
    main()
