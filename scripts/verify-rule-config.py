import argparse
import ipaddress
import json
from pathlib import Path
import socket
import struct
import subprocess
import tempfile
import time


def dns_query(server, name, request_id, deadline, tcp=False):
    question = b"".join(bytes([len(label)]) + label.encode("ascii") for label in name.split("."))
    question += b"\0" + struct.pack("!HH", 1, 1)
    query = struct.pack("!6H", request_id, 0x0100, 1, 0, 0, 0) + question
    while True:
        try:
            kind = socket.SOCK_STREAM if tcp else socket.SOCK_DGRAM
            with socket.socket(socket.AF_INET, kind) as client:
                client.settimeout(0.2)
                client.connect(server)
                if tcp:
                    client.sendall(struct.pack("!H", len(query)) + query)
                    with client.makefile("rb") as stream:
                        length, = struct.unpack("!H", stream.read(2))
                        reply = stream.read(length)
                        assert len(reply) == length
                else:
                    client.send(query)
                    reply = client.recv(2048)
            break
        except (socket.timeout, ConnectionResetError, ConnectionRefusedError):
            if time.monotonic() >= deadline:
                raise
    response_id, flags, questions, answers, _, _ = struct.unpack_from("!6H", reply)
    assert response_id == request_id and flags & 0x8000 and questions == 1
    assert reply[12:12 + len(question)] == question
    return flags & 15, answers, reply


def assert_rejected(command, directory, config, error_text):
    result = subprocess.run(
        [*command, "start", "-c", config], cwd=directory,
        capture_output=True, timeout=10, check=False,
    )
    assert result.returncode != 0 and error_text in result.stderr, result.stderr


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("binary", type=Path)
    parser.add_argument("--emulator")
    args = parser.parse_args()
    command = [str(args.binary.resolve())]
    if args.emulator:
        command.insert(0, args.emulator)
    examples = Path(__file__).resolve().parents[1] / "examples"
    with tempfile.TemporaryDirectory(prefix="mosdns-validation-", dir=Path.cwd()) as temp:
        directory = Path(temp)
        (directory / "examples").mkdir()
        for name in ("domains", "ips"):
            subprocess.run(
                [*command, "rule-set", "compile", "-o", f"examples/{name}.mdrs",
                 str(examples / f"{name}.json")],
                cwd=directory, timeout=15, check=True,
            )
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]

        config = json.loads((examples / "config.json").read_text(encoding="utf-8"))
        config["plugins"][-1]["args"]["listen"] = f"127.0.0.1:{port}"
        config["plugins"].append({"type": "tcp_server", "args": config["plugins"][-1]["args"]})
        config["plugins"][:0] = [
            {"tag": "ordinary_cache", "type": "cache", "args": {"size": 65535}},
            {"tag": "node_cache", "type": "cache", "args": {"size": 1}},
        ]
        (directory / "rules.json").write_text(json.dumps(config), encoding="utf-8")
        (directory / "config.json").write_text('{"include": ["rules.json"]}', encoding="utf-8")
        with (directory / "server.log").open("wb+") as log:
            process = subprocess.Popen([*command, "start"], cwd=directory, stdout=log, stderr=log)
            try:
                for tcp in (False, True):
                    for request_id, (name, rcode) in enumerate([
                        ("sub.blocked.example.test", 5),
                        ("ip.example.test", 5),
                        ("allowed.example.test", 0),
                    ], start=1):
                        actual, count, reply = dns_query(
                            ("127.0.0.1", port), name, request_id, time.monotonic() + 5, tcp=tcp,
                        )
                        assert actual == rcode, (name, tcp, actual)
                        if rcode == 5:
                            assert count == 0
                        else:
                            assert count == 1
                            assert reply.endswith(b"\0\x04" + ipaddress.ip_address("198.51.100.10").packed)
            except (AssertionError, OSError):
                log.seek(0)
                print(log.read().decode("utf-8", errors="replace"))
                raise
            finally:
                if process.poll() is None:
                    process.terminate()
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait(timeout=5)

        (directory / "config.yaml").write_text("plugins: []", encoding="utf-8")
        assert_rejected(command, directory, "config.yaml", b"configuration must be a .json file")
        (directory / "examples" / "domains.mdrs").write_text("blocked.example.test\n", encoding="utf-8")
        assert_rejected(command, directory, "config.json", b"invalid MosDNS binary rule file header")


if __name__ == "__main__":
    main()
