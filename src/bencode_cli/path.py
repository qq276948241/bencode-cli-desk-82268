from typing import Any, List, Tuple

from .errors import BencodeError


def get_path(value: Any, path: str) -> Any:
    current = value
    for kind, token in parse_path(path):
        if kind == "key":
            if not isinstance(current, dict):
                raise LookupError(path)
            try:
                key = token.encode("utf-8")
            except UnicodeEncodeError as exc:
                raise BencodeError("path key must be valid UTF-8") from exc
            if key not in current:
                raise LookupError(path)
            current = current[key]
        else:
            if not isinstance(current, list):
                raise LookupError(path)
            index = int(token)
            if index >= len(current):
                raise LookupError(path)
            current = current[index]
    return current


def parse_path(path: str) -> List[Tuple[str, str]]:
    if not path:
        return []

    tokens = []
    cursor = 0
    if not path.startswith("["):
        cursor = _consume_bare_key(path, 0, tokens)

    while cursor < len(path):
        char = path[cursor]
        if char == ".":
            cursor = _consume_bare_key(path, cursor + 1, tokens)
        elif char == "[":
            end = path.find("]", cursor + 1)
            if end == -1:
                raise BencodeError("path is missing ']'")
            content = path[cursor + 1:end]
            if content and content[0] in {'"', "'"}:
                if len(content) < 2 or content[-1] != content[0]:
                    raise BencodeError("unterminated quoted path key")
                tokens.append(("key", content[1:-1]))
            else:
                if not content or not content.isdigit():
                    raise BencodeError("list index in brackets must be a non-negative integer")
                tokens.append(("index", content))
            cursor = end + 1
        else:
            raise BencodeError(f"unexpected character {char!r} in path")
    return tokens


def _consume_bare_key(path: str, start: int, tokens) -> int:
    if start >= len(path) or path[start] == ".":
        raise BencodeError("empty path key")
    end = start
    while end < len(path) and path[end] not in ".[":
        end += 1
    tokens.append(("key", path[start:end]))
    return end

