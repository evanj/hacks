// zlibpipe compresses or decompresses stdin to stdout using zlib.
package main

import (
	"compress/zlib"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	decompress := flag.Bool("decompress", false, "decompress instead of compressing")
	flag.Parse()

	var reader io.ReadCloser = os.Stdin
	var writer io.WriteCloser = os.Stdout

	opName := "compressing"
	if *decompress {
		opName = "decompressing"

		zlibReader, err := zlib.NewReader(reader)
		if err != nil {
			panic(err)
		}
		reader = zlibReader
	} else {
		zlibWriter, err := zlib.NewWriterLevel(writer, zlib.BestCompression)
		if err != nil {
			panic(err)
		}
		writer = zlibWriter
	}

	fmt.Fprintf(os.Stderr, "zlibpipe: %s from stdin to stdout ...\n", opName)

	n, err := io.Copy(writer, reader)
	if err != nil {
		panic(err)
	}
	err = writer.Close()
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "zlibpipe: processed %d bytes\n", n)
}
