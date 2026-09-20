import argparse
import hashlib
import sys

from . import __version__
from .codec import dump_intermediate_json, encode, load_intermediate_json
from .errors import BencodeError
from .parser import parse
from .path import get_path


EPILOG = """\
path syntax:
  info.name                 dict key
  info.files[0].path        list index followed by dict key
  info["my key"]            quoted dict key

intermediate JSON:
  integers  {"type":"integer","value":"42"}
  bytes     {"type":"bytes","hex":"6f6b"}
  lists     {"type":"list","items":[...]}
  dicts     {"type":"dict","entries":[{"key":{...},"value":{...}}]}
"""


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="bencode",
        description="Decode, query, hash, and encode bencode files such as .torrent metainfo.",
        epilog=EPILOG,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("--version", action="version", version=f"bencode {__version__}")
    subparsers = parser.add_subparsers(dest="command", required=True)

    decode_parser = subparsers.add_parser(
        "decode", help="pretty-print bencode as the documented JSON intermediate form"
    )
    decode_parser.add_argument("file", help="bencode file, or '-' for stdin")
    decode_parser.add_argument("--max-depth", type=int, default=100)

    get_parser = subparsers.add_parser("get", help="select one value by path")
    get_parser.add_argument("file", help="bencode file, or '-' for stdin")
    get_parser.add_argument("path", help="field path, for example info.files[0].path")
    get_parser.add_argument("--max-depth", type=int, default=100)

    hash_parser = subparsers.add_parser(
        "hash", help="print the lowercase hex SHA-1 infohash of the root info dict"
    )
    hash_parser.add_argument("file", help=".torrent or other bencode file, or '-' for stdin")
    hash_parser.add_argument("--max-depth", type=int, default=100)

    encode_parser = subparsers.add_parser(
        "encode", help="encode the documented intermediate JSON back to bencode"
    )
    encode_parser.add_argument("input", help="intermediate JSON file, or '-' for stdin")
    encode_parser.add_argument("-o", "--output", required=True, help="output bencode file")

    return parser


def main(argv=None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        if args.command == "decode":
            parsed = parse(_read_bytes(args.file), args.max_depth)
            sys.stdout.write(dump_intermediate_json(parsed.value))
        elif args.command == "get":
            parsed = parse(_read_bytes(args.file), args.max_depth)
            try:
                selected = get_path(parsed.value, args.path)
            except LookupError:
                raise BencodeError(f"path not found: {args.path}")
            sys.stdout.write(dump_intermediate_json(selected))
        elif args.command == "hash":
            raw = _read_bytes(args.file)
            parsed = parse(raw, args.max_depth)
            if parsed.info_span is None:
                raise BencodeError("root bencode value has no 'info' dict")
            start, end = parsed.info_span
            sys.stdout.write(hashlib.sha1(raw[start:end]).hexdigest() + "\n")
        elif args.command == "encode":
            value = load_intermediate_json(_read_text(args.input))
            with open(args.output, "wb") as output:
                output.write(encode(value))
        return 0
    except (BencodeError, OSError, UnicodeError) as exc:
        print(f"bencode: error: {exc}", file=sys.stderr)
        return 1


def _read_bytes(path: str) -> bytes:
    if path == "-":
        return sys.stdin.buffer.read()
    with open(path, "rb") as source:
        return source.read()


def _read_text(path: str) -> str:
    if path == "-":
        return sys.stdin.read()
    with open(path, "r", encoding="utf-8") as source:
        return source.read()

