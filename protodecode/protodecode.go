package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/richardartoul/molecule"
	"github.com/richardartoul/molecule/src/codec"
	"google.golang.org/protobuf/encoding/protowire"
)

func main() {
	nestedPaths := flag.String("nested", "", "a comma (,) separated list of tag paths for nested messages e.g. '1,2.3'")
	tagsOnly := flag.Bool("tagsOnly", false, "if true, will only decode tags (field number, wire type) at each byte offset; useful for finding the start of a message")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: protodecode (path)")
		os.Exit(1)
	}
	inputPath := flag.Arg(0)

	nestedSet, err := parseNested(*nestedPaths)
	if err != nil {
		panic(err)
	}

	protoBytes, err := os.ReadFile(inputPath)
	if err != nil {
		panic(err)
	}

	if *tagsOnly {
		err = decodeTags(protoBytes)
	} else {
		err = decode(os.Stdout, protoBytes, nestedSet)
	}
	if err != nil {
		panic(err)
	}
}

var wireTypes = map[codec.WireType]string{
	codec.WireVarint:     "varint",
	codec.WireFixed64:    "fixed64",
	codec.WireBytes:      "length-delimited",
	codec.WireStartGroup: "start-group",
	codec.WireEndGroup:   "end-group",
	codec.WireFixed32:    "fixed32",
}

const replacementChar = '.'

func printableUTF8(b []byte) string {
	output := strings.Builder{}

	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.C, r) {
			// non-printable Unicode characters
			for i := 0; i < n; i++ {
				output.WriteByte(replacementChar)
			}
		} else {
			output.WriteRune(r)
		}
		b = b[n:]
	}
	return output.String()
}

type nestedPathsSet map[string]struct{}

func parseNested(specification string) (nestedPathsSet, error) {
	if specification == "" {
		return nil, nil
	}

	set := nestedPathsSet{}
	paths := strings.Split(specification, ",")
	for _, path := range paths {
		// validate that path is a sequence of . separated integers
		parts := strings.Split(path, ".")
		for i, part := range parts {
			v, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			if v <= 0 {
				return nil, fmt.Errorf("invalid tag in nested specification: %d", v)
			}

			// ensure that we include all parent paths in the set
			partialPath := strings.Join(parts[:i+1], ".")
			set[partialPath] = struct{}{}
		}

		// set[path] = struct{}{}
	}
	return set, nil
}

func pathForField(path string, fieldNum int32) string {
	fieldPath := strconv.Itoa(int(fieldNum))
	if path != "" {
		fieldPath = path + "." + fieldPath
	}
	return fieldPath
}

func fieldIsNested(nested nestedPathsSet, path string, fieldNum int32) bool {
	_, exists := nested[pathForField(path, fieldNum)]
	return exists
}

func decode(w io.Writer, buf []byte, nested nestedPathsSet) error {
	return decodeRecursive(w, buf, nested, 0, "")
}

func decodeRecursive(w io.Writer, buf []byte, nested nestedPathsSet, offset int, path string) error {
	depth := 0
	if path != "" {
		depth = 1 + strings.Count(path, ".")
	}
	depthPrefix := ""
	for i := 0; i < depth; i++ {
		depthPrefix += "  "
	}

	cb := codec.NewBuffer(buf)
	lastOffset := len(buf) - cb.Len()
	err := molecule.MessageEach(cb, func(fieldNum int32, value molecule.Value) (bool, error) {
		nextOffset := len(buf) - cb.Len()
		fmt.Fprintf(w, "%sbytes %d-%d: field=%d type=%d (%s)",
			depthPrefix, lastOffset+offset, nextOffset+offset, fieldNum,
			value.WireType, wireTypes[value.WireType])

		decodeNested := false
		switch value.WireType {
		case codec.WireVarint, codec.WireFixed64, codec.WireFixed32:
			fmt.Fprintf(w, " uint=%d", value.Number)

		case codec.WireBytes:
			fmt.Fprintf(w, " len=%d", len(value.Bytes))
			if fieldIsNested(nested, path, fieldNum) {
				fmt.Fprintf(w, " nested message")
				decodeNested = true
			} else {
				fmt.Fprintf(w, " str=%#v hex=%s",
					printableUTF8(value.Bytes), hex.EncodeToString(value.Bytes))
			}

		default:
			panic("TODO")
		}

		// TODO: add hex encoding for the raw tag?
		fmt.Fprintf(w, "\n")

		if decodeNested {
			// TODO: fix the offset
			msgStart := nextOffset - len(value.Bytes)
			err := decodeRecursive(w, value.Bytes, nested, msgStart, pathForField(path, fieldNum))
			if err != nil {
				return false, err
			}
		}

		// if fieldNum == 1 {
		// 	// decode a sub-message
		// 	cb2 := codec.NewBuffer(value.Bytes)
		// 	lastNested := len(value.Bytes) - cb2.Len()
		// 	err := molecule.MessageEach(cb2, func(fieldNum int32, v2 molecule.Value) (bool, error) {
		// 		nextNested := len(value.Bytes) - cb2.Len()
		// 		fmt.Printf("  bytes %d-%d: field=%d type=%d %s number=%d len(bytes)=%d\n",
		// 			lastOffset+lastNested, lastOffset+nextNested, fieldNum, v2.WireType, wireTypes[value.WireType], v2.Number, len(v2.Bytes))
		// 		lastNested = nextNested
		// 		return true, nil
		// 	})
		// 	if err != nil {
		// 		fmt.Printf("  err decoding nested message: %s", err.Error())
		// 	}
		// }

		lastOffset = nextOffset
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("decode failed at offset %d: %w", len(buf)-cb.Len()+offset, err)
	}
	return nil
}

func decodeTags(buf []byte) error {
	// try decoding at every byte offset
	for i := 0; i < len(buf); i++ {
		cb := codec.NewBuffer(buf[i:])
		v, err := cb.DecodeVarint()
		if err != nil {
			panic("varint decoding failed: " + err.Error())
		}

		// TODO: check the error? We expect errors, so the only thing we could do is log it
		// for now, we ignore the error
		fieldNum, wireType, _ := codec.AsTagAndWireType(v)
		encodedVarint := protowire.AppendVarint(nil, v)
		fmt.Printf("offset %d: 0x%s varint=%d; field num=%d; wire type=%d %s",
			len(buf)-cb.Len()-len(encodedVarint), hex.EncodeToString(encodedVarint),
			v, fieldNum, wireType, wireTypes[wireType])

		// if this looks like a valid start of a protocol buffer message, start decoding here
		invalid := false
		if fieldNum <= 0 {
			invalid = true
			fmt.Printf(" invalid: field num must be > 0")
		}
		if wireTypes[wireType] == "" {
			invalid = true
			fmt.Printf(" invalid: unknown wire type")
		} else if wireType == codec.WireStartGroup || wireType == codec.WireEndGroup {
			invalid = true
			fmt.Printf(" invalid: group wire types are deprecated")
		}
		if !invalid {
			fmt.Printf(" VALID!\n")
		} else {
			fmt.Println()
		}
	}
	return nil
}
