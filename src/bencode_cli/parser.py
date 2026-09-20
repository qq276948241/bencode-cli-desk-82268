from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Tuple

from .errors import BencodeError


@dataclass(frozen=True)
class ParsedData:
    value: Any
    info_span: Optional[Tuple[int, int]]


def parse(data: bytes, max_depth: int = 100) -> ParsedData:
    if not isinstance(data, (bytes, bytearray, memoryview)):
        raise TypeError("bencode input must be bytes-like")
    if max_depth < 0:
        raise BencodeError("maximum depth must be non-negative")
    if len(data) == 0:
        raise BencodeError("empty bencode input", 0)

    value, end, info_span = _parse_value(bytes(data), 0, 0, max_depth)
    if end != len(data):
        raise BencodeError("unexpected trailing data", end)
    return ParsedData(value=value, info_span=info_span)


def _parse_value(data: bytes, pos: int, depth: int, max_depth: int):
    if pos >= len(data):
        raise BencodeError("expected a bencode value", pos)
    if depth > max_depth:
        raise BencodeError(f"maximum nesting depth ({max_depth}) exceeded", pos)

    marker = data[pos]
    if marker == 0x69:
        return _parse_int(data, pos)
    if marker == 0x6C:
        return _parse_list(data, pos, depth, max_depth)
    if marker == 0x64:
        return _parse_dict(data, pos, depth, max_depth)
    if 0x30 <= marker <= 0x39:
        return _parse_bytes(data, pos)
    raise BencodeError("invalid bencode type marker", pos)


def _parse_int(data: bytes, pos: int):
    end = data.find(b"e", pos + 1)
    if end == -1:
        raise BencodeError("unterminated integer", pos)

    number = data[pos + 1:end]
    if not number:
        raise BencodeError("empty integer", pos + 1)
    if number == b"0":
        return 0, end + 1, None
    if number[0] != 0x2D and number[0] != 0x30:
        try:
            return int(number), end + 1, None
        except ValueError:
            pass
    if number[0] == 0x2D and len(number) > 1 and number[1] != 0x30:
        try:
            return int(number), end + 1, None
        except ValueError:
            pass
    raise BencodeError("invalid integer encoding", pos + 1)


def _parse_bytes(data: bytes, pos: int):
    colon = data.find(b":", pos)
    if colon == -1:
        raise BencodeError("missing byte-string length separator", pos)

    length_text = data[pos:colon]
    if not length_text or not all(0x30 <= byte <= 0x39 for byte in length_text):
        raise BencodeError("invalid byte-string length prefix", pos)
    if len(length_text) > 1 and length_text[0] == 0x30:
        raise BencodeError("leading zero in byte-string length", pos)

    length = int(length_text)
    start = colon + 1
    end = start + length
    if length > len(data) - start:
        raise BencodeError("byte-string length exceeds available input", pos)
    return data[start:end], end, None


def _parse_list(data: bytes, pos: int, depth: int, max_depth: int):
    if depth + 1 > max_depth:
        raise BencodeError(f"maximum nesting depth ({max_depth}) exceeded", pos)
    cursor = pos + 1
    items: List[Any] = []
    while True:
        if cursor >= len(data):
            raise BencodeError("unterminated list", pos)
        if data[cursor] == 0x65:
            return items, cursor + 1, None
        item, cursor, _ = _parse_value(data, cursor, depth + 1, max_depth)
        items.append(item)


def _parse_dict(data: bytes, pos: int, depth: int, max_depth: int):
    if depth + 1 > max_depth:
        raise BencodeError(f"maximum nesting depth ({max_depth}) exceeded", pos)
    cursor = pos + 1
    result: Dict[bytes, Any] = {}
    info_span = None

    while True:
        if cursor >= len(data):
            raise BencodeError("unterminated dict", pos)
        if data[cursor] == 0x65:
            return result, cursor + 1, info_span

        key_start = cursor
        key, cursor, _ = _parse_bytes(data, cursor)
        if key in result:
            raise BencodeError(f"duplicate dict key {key!r}", key_start)

        value_start = cursor
        value, cursor, nested_info = _parse_value(data, cursor, depth + 1, max_depth)
        result[key] = value

        if depth == 0 and key == b"info" and isinstance(value, dict):
            info_span = (value_start, cursor)
        elif nested_info is not None:
            info_span = nested_info
