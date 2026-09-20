package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"bencode-cli/bencode"
)

const usage = `bencode - inspect and manipulate .torrent / bencode files

USAGE:
  bencode <command> [flags] [file]

  Read from the given file, or from stdin when file is "-" or omitted.

COMMANDS:
  dump      Pretty-print a bencoded file as a readable tree
  get       Extract one field by path; exits non-zero if absent
  hash      Print the SHA-1 infohash of the info dict (lowercase hex)
  encode    Convert intermediate JSON (see FORMAT) back into bencode

DUMP FLAGS:
  --style auto|hex|ascii   Byte-string rendering (default: auto)
                             auto  : valid UTF-8 shown quoted, else hex
                             hex   : all byte strings as 0x...
                             ascii : printable ASCII quoted, else hex

GET FLAGS:
  --style auto|hex|ascii   Rendering for byte-string results (default: auto)
  --raw                    Print the exact bencoded bytes of the field

PATH SYNTAX (get):
  Dot-separated dict keys with optional zero-based [index] list accessors.
  Examples:
    info.name
    info.files[0].path
    info.piece length
    announce-list[1][0]

INTERMEDIATE JSON FORMAT (decode output / encode input):
  - integers become JSON numbers (must be whole integers)
  - byte strings that are valid UTF-8 become JSON strings
  - non-UTF-8 byte strings become {"@hex": "<lowercase hex>"}
  - lists become JSON arrays
  - dicts become JSON objects

EXAMPLES:
  bencode dump ubuntu.torrent
  bencode get info.name ubuntu.torrent
  bencode get --raw info.pieces ubuntu.torrent > pieces.bin
  bencode hash ubuntu.torrent
  bencode dump --style hex ubuntu.torrent > tree.txt
  bencode encode tree.json > rebuilt.torrent
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Print(usage)
		return
	}
	args := os.Args[2:]
	var err error
	switch cmd {
	case "dump":
		err = runDump(args)
	case "get":
		err = runGet(args)
	case "hash":
		err = runHash(args)
	case "encode":
		err = runEncode(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "bencode %s: %s\n", cmd, err.Error())
		os.Exit(1)
	}
}

type commonFlags struct {
	style string
	raw   bool
	files []string
}

func parseFlags(args []string, allowed map[string]bool) (*commonFlags, error) {
	f := &commonFlags{style: "auto"}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--style":
			if i+1 >= len(args) {
				return nil, errors.New("--style requires a value (auto|hex|ascii)")
			}
			i++
			f.style = args[i]
		case allowed["raw"] && a == "--raw":
			f.raw = true
		case a == "-h" || a == "--help":
			fmt.Print(usage)
			os.Exit(0)
		case len(a) > 2 && a[:2] == "--":
			return nil, fmt.Errorf("unknown flag %q", a)
		default:
			f.files = append(f.files, a)
		}
	}
	return f, nil
}

func readInput(files []string) ([]byte, error) {
	if len(files) == 0 || files[0] == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", files[0], err)
	}
	return data, nil
}

func decodeInput(files []string) (*bencode.Value, []byte, error) {
	data, err := readInput(files)
	if err != nil {
		return nil, nil, err
	}
	if len(data) == 0 {
		return nil, nil, errors.New("input is empty")
	}
	v, err := bencode.Decode(data)
	if err != nil {
		return nil, nil, err
	}
	return v, data, nil
}

func parseStyle(s string) (bencode.StringStyle, error) {
	switch s {
	case "auto":
		return bencode.StyleAuto, nil
	case "hex":
		return bencode.StyleHex, nil
	case "ascii":
		return bencode.StyleASCII, nil
	default:
		return 0, fmt.Errorf("invalid --style %q (want auto|hex|ascii)", s)
	}
}

func runDump(args []string) error {
	f, err := parseFlags(args, map[string]bool{})
	if err != nil {
		return err
	}
	if len(f.files) > 1 {
		return fmt.Errorf("unexpected extra argument %q", f.files[1])
	}
	style, err := parseStyle(f.style)
	if err != nil {
		return err
	}
	v, _, err := decodeInput(f.files)
	if err != nil {
		return err
	}
	fmt.Print(bencode.Render(v, style))
	return nil
}

func runGet(args []string) error {
	f, err := parseFlags(args, map[string]bool{"raw": true})
	if err != nil {
		return err
	}
	if len(f.files) == 0 {
		return errors.New("get requires a path, e.g. bencode get info.name file.torrent")
	}
	if len(f.files) > 2 {
		return fmt.Errorf("unexpected extra argument %q", f.files[2])
	}
	// First positional is the path; remaining positional (if any) is the file.
	path := f.files[0]
	var fileArg []string
	if len(f.files) == 2 {
		fileArg = f.files[1:]
	}
	data, err := readInput(fileArg)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("input is empty")
	}
	root, err := bencode.Decode(data)
	if err != nil {
		return err
	}
	field, err := bencode.Get(root, path)
	if err != nil {
		return err
	}
	if f.raw {
		os.Stdout.Write(field.Raw)
		if len(field.Raw) > 0 && field.Raw[len(field.Raw)-1] != '\n' {
			fmt.Println()
		}
		return nil
	}
	style, err := parseStyle(f.style)
	if err != nil {
		return err
	}
	fmt.Print(bencode.Render(field, style))
	return nil
}

func runHash(args []string) error {
	f, err := parseFlags(args, map[string]bool{})
	if err != nil {
		return err
	}
	if len(f.files) > 1 {
		return fmt.Errorf("unexpected extra argument %q", f.files[1])
	}
	v, _, err := decodeInput(f.files)
	if err != nil {
		return err
	}
	h, err := bencode.InfoHash(v)
	if err != nil {
		return err
	}
	fmt.Println(h)
	return nil
}

func runEncode(args []string) error {
	f, err := parseFlags(args, map[string]bool{})
	if err != nil {
		return err
	}
	if len(f.files) > 1 {
		return fmt.Errorf("unexpected extra argument %q", f.files[1])
	}
	data, err := readInput(f.files)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("input JSON is empty")
	}
	var v *bencode.Value
	// Accept either the tagged intermediate JSON via FromJSON.
	v, err = bencode.FromJSON(data)
	if err != nil {
		return err
	}
	out, err := bencode.Encode(v)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	return nil
}
