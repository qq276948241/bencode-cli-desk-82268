# bencode-cli

一个只依赖 Python 标准库的小型 bencode / `.torrent` CLI。它可以解析、按路径查询、计算 BitTorrent info dict 的 SHA-1 infohash，并把明确类型的 JSON 中间表示重新编码为 bencode。

## 安装

要求 Python 3.9+。

```bash
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -e .
bencode --help
```

不想安装时也可以直接在仓库根目录运行：

```bash
PYTHONPATH=src python3 -m bencode_cli --help
```

## 输出与 JSON 中间格式

`decode` 和 `get` 输出 JSON，所有 bencode 字节串都保留为二进制安全的 `hex`，可打印 ASCII（`0x20` 到 `0x7E`）会额外附带 `ascii` 字段：

```json
{
  "type": "bytes",
  "hex": "64656d6f2e62696e",
  "ascii": "demo.bin"
}
```

其他节点格式：

- 整数：`{"type":"integer","value":"42"}`
- 列表：`{"type":"list","items":[...]}`
- 字典：`{"type":"dict","entries":[{"key":{...},"value":{...}}]}`

字典在 JSON 中使用 `entries` 数组展示原始键顺序；重新编码时会按 bencode 的字节键字典序规范化输出。

## 路径语法

- `info.name`：字典键
- `info.files[0].path`：列表下标加字典键
- `info["my key"]`：带空格或特殊字符的引号键

路径不存在、类型不匹配或下标越界时，`get` 返回非零退出码。

## 常用命令

漂亮打印整个文件：

```bash
bencode decode example.torrent
```

读取嵌套字段：

```bash
bencode get example.torrent info.name
bencode get example.torrent info.files[0].path
```

计算磁力链使用的小写十六进制 infohash：

```bash
bencode hash example.torrent
```

把 JSON 中间表示重新编码：

```bash
bencode decode example.torrent > decoded.json
bencode encode decoded.json -o rebuilt.torrent
bencode hash rebuilt.torrent
```

对于标准单文件种子，上面的 `rebuilt.torrent` 与 `example.torrent` 应得到相同 infohash。

## 三条验收命令

把下面的 `example.torrent` 替换成任意已有种子：

```bash
bencode get example.torrent info.name
bencode hash example.torrent
bencode decode example.torrent > decoded.json && bencode encode decoded.json -o rebuilt.torrent && bencode hash rebuilt.torrent
```

第三条输出的 infohash 应与第二条一致。也可以运行自动化测试：

```bash
PYTHONPATH=src python3 -m unittest discover -s tests -v
```

## 解析边界

- 空文件、非法类型标记、非法整数和非法字节串长度前缀都会返回清晰错误和非零退出码。
- 超长字节串按实际剩余字节数拒绝，不会按伪造长度分配大内存。
- 默认最大嵌套深度为 100，可用 `--max-depth` 调整。
- 拒绝重复 dict 键、未消费的尾部数据和非标准前导零。

