import json
from typing import Any, Dict

from .errors import BencodeError


def encode(value: Any) -> bytes:
    if isinstance(value, bool):
        raise BencodeError("JSON booleans have no bencode representation")
    if isinstance(value, int):
        return b"i" + str(value).encode("ascii") + b"e"
    if isinstance(value, bytes):
        return str(len(value)).encode("ascii") + b":" + value
    if isinstance(value, list):
        return b"l" + b"".join(encode(item) for item in value) + b"e"
    if isinstance(value, dict):
        try:
            items = [(encode(key), encode(value[key])) for key in sorted(value)]
        except TypeError as exc:
            raise BencodeError("dict keys must be byte strings") from exc
        return b"d" + b"".join(key + item for key, item in items) + b"e"
    raise BencodeError(f"unsupported bencode value type: {type(value).__name__}")


def to_intermediate(value: Any) -> Any:
    if isinstance(value, bool):
        raise BencodeError("booleans cannot occur in decoded bencode")
    if isinstance(value, int):
        return {"type": "integer", "value": str(value)}
    if isinstance(value, bytes):
        node = {"type": "bytes", "hex": value.hex()}
        if all(0x20 <= byte <= 0x7E for byte in value):
            node["ascii"] = value.decode("ascii")
        return node
    if isinstance(value, list):
        return {"type": "list", "items": [to_intermediate(item) for item in value]}
    if isinstance(value, dict):
        return {
            "type": "dict",
            "entries": [
                {"key": to_intermediate(key), "value": to_intermediate(item)}
                for key, item in value.items()
            ],
        }
    raise BencodeError(f"unsupported decoded value type: {type(value).__name__}")


def from_intermediate(node: Any) -> Any:
    if not isinstance(node, dict) or "type" not in node:
        raise BencodeError("intermediate JSON nodes must include a string 'type' field")

    node_type = node.get("type")
    if node_type == "integer":
        text = node.get("value")
        if not isinstance(text, str) or not _is_integer_text(text):
            raise BencodeError("integer node requires a decimal string 'value'")
        return int(text)
    if node_type == "bytes":
        return _bytes_from_node(node)
    if node_type == "list":
        items = node.get("items")
        if not isinstance(items, list):
            raise BencodeError("list node requires an 'items' array")
        return [from_intermediate(item) for item in items]
    if node_type == "dict":
        return _dict_from_node(node)
    raise BencodeError(f"unknown intermediate node type: {node_type!r}")


def load_intermediate_json(text: str) -> Any:
    try:
        decoded = json.loads(text)
    except json.JSONDecodeError as exc:
        raise BencodeError(f"invalid intermediate JSON: {exc.msg}", exc.pos) from exc
    return from_intermediate(decoded)


def dump_intermediate_json(value: Any) -> str:
    return json.dumps(to_intermediate(value), ensure_ascii=False, indent=2) + "\n"


def _bytes_from_node(node: Dict[str, Any]) -> bytes:
    hex_value = node.get("hex")
    if not isinstance(hex_value, str):
        raise BencodeError("bytes node requires a lowercase 'hex' string")
    try:
        value = bytes.fromhex(hex_value)
    except ValueError as exc:
        raise BencodeError("bytes node has invalid hex data") from exc
    if value.hex() != hex_value:
        raise BencodeError("bytes hex must be lowercase and use full bytes")
    return value


def _dict_from_node(node: Dict[str, Any]) -> Dict[bytes, Any]:
    entries = node.get("entries")
    if not isinstance(entries, list):
        raise BencodeError("dict node requires an 'entries' array")

    result = {}
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict):
            raise BencodeError(f"dict entry {index} must be an object")
        key = from_intermediate(entry.get("key"))
        if not isinstance(key, bytes):
            raise BencodeError(f"dict entry {index} key must be a bytes node")
        if key in result:
            raise BencodeError(f"duplicate intermediate dict key {key!r}")
        result[key] = from_intermediate(entry.get("value"))
    return result


def _is_integer_text(text: str) -> bool:
    if text == "0":
        return True
    if text.startswith("-"):
        return len(text) > 1 and text[1:].isdigit() and text[1] != "0"
    return text.isdigit() and text[0] != "0"

