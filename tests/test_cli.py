import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from bencode_cli.codec import dump_intermediate_json, encode, load_intermediate_json
from bencode_cli.errors import BencodeError
from bencode_cli.parser import parse


class BencodeTests(unittest.TestCase):
    def test_roundtrip_and_infohash(self):
        pieces = bytes(range(256)) * 2
        torrent = {
            b"announce": b"http://tracker.example/announce",
            b"info": {
                b"name": b"demo.bin",
                b"length": 512,
                b"piece length": 32768,
                b"pieces": pieces,
                b"private": 1,
            },
        }
        raw = encode(torrent)
        parsed = parse(raw)

        self.assertEqual(encode(parsed.value), raw)

        info_encoded = (
            b"d4:infod6:lengthi512e4:name8:demo.bin"
            b"12:piece lengthi32768e6:pieces512:" + pieces + b"7:privatei1eee"
        )
        expected_info = info_encoded[7:-1]
        start, end = parsed.info_span
        self.assertEqual(raw[start:end], expected_info)
        self.assertEqual(
            hashlib.sha1(raw[start:end]).hexdigest(),
            hashlib.sha1(encode(parsed.value[b"info"])).hexdigest(),
        )

    def test_intermediate_json_roundtrip(self):
        value = {b"k": [b"ok", -7, {}]}
        text = dump_intermediate_json(value)
        self.assertEqual(encode(load_intermediate_json(text)), encode(value))
        self.assertIn('"hex": "6f6b"', text)
        self.assertIn('"ascii": "ok"', text)

    def test_invalid_length_prefix_is_rejected(self):
        for raw in [b"", b"x", b"i01e", b"i-0e", b"03:abc", b"99:abc", b"3abc"]:
            with self.subTest(raw=raw):
                with self.assertRaises(BencodeError):
                    parse(raw)

    def test_depth_limit_is_enforced(self):
        with self.assertRaises(BencodeError):
            parse(b"l" * 101 + b"e" * 101, max_depth=100)


class CliTests(unittest.TestCase):
    def run_cli(self, *args):
        return subprocess.run(
            [sys.executable, "-m", "bencode_cli", *args],
            cwd=Path(__file__).resolve().parents[1],
            env={"PYTHONPATH": str(Path(__file__).resolve().parents[1] / "src"), "PATH": sys.executable.rsplit("/", 1)[0]},
            text=True,
            capture_output=True,
        )

    def test_get_hash_encode_commands(self):
        torrent = {b"info": {b"name": b"demo.bin", b"length": 4, b"pieces": b""}}
        raw = encode(torrent)
        with tempfile.TemporaryDirectory() as temp_dir:
            source = Path(temp_dir) / "demo.torrent"
            decoded = Path(temp_dir) / "decoded.json"
            rebuilt = Path(temp_dir) / "rebuilt.torrent"
            source.write_bytes(raw)

            result = self.run_cli("decode", str(source))
            self.assertEqual(result.returncode, 0, result.stderr)
            decoded.write_text(result.stdout)

            result = self.run_cli("get", str(source), "info.name")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)["ascii"], "demo.bin")

            missing = self.run_cli("get", str(source), "info.missing")
            self.assertNotEqual(missing.returncode, 0)

            original_hash = self.run_cli("hash", str(source))
            self.assertEqual(original_hash.returncode, 0, original_hash.stderr)

            result = self.run_cli("encode", str(decoded), "-o", str(rebuilt))
            self.assertEqual(result.returncode, 0, result.stderr)
            rebuilt_hash = self.run_cli("hash", str(rebuilt))
            self.assertEqual(rebuilt_hash.stdout, original_hash.stdout)


if __name__ == "__main__":
    unittest.main()
