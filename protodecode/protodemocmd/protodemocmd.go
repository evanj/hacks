package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/evanj/hacks/protodecode/protodemo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	outPath := flag.String("out", "", "path to write binary protocol buffer data")
	flag.Parse()

	demoMsg := &protodemo.DecodeDemo{}
	demoMsg.Int64Value = math.MinInt64
	demoMsg.StringValue = "Héllo 🌎!"
	demoMsg.Timestamp = &timestamppb.Timestamp{Seconds: 1607863096, Nanos: 437553000}
	out, err := proto.Marshal(demoMsg)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Go string: %#v\nHex: %s\n", string(out), hex.EncodeToString(out))

	if *outPath != "" {
		fmt.Printf("writing to %s ...\n", *outPath)
		err = os.WriteFile(*outPath, out, 0600)
		if err != nil {
			panic(err)
		}
	}
}
