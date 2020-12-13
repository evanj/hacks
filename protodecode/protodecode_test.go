package main

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/evanj/hacks/protodecode/protodemo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDecode(t *testing.T) {
	tsProto := &timestamppb.Timestamp{Seconds: 1607863096, Nanos: 437553000}
	tsAny, err := anypb.New(tsProto)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		input          *protodemo.DecodeDemo
		nested         string
		expectedSubstr string
	}{
		{"empty", &protodemo.DecodeDemo{}, "", ""},
		{"MaxInt64", &protodemo.DecodeDemo{Int64Value: math.MaxInt64}, "", "uint=9223372036854775807"},
		{"MinInt64", &protodemo.DecodeDemo{Int64Value: math.MinInt64}, "", "uint=9223372036854775808"},
		{"UnicodeStr", &protodemo.DecodeDemo{StringValue: "Héllo 🌎!"}, "", `len=12 str="Héllo 🌎!"`},
		{"Bytes", &protodemo.DecodeDemo{BytesValue: []byte("Hé\xff\x00o")}, "", `len=6 str="Hé..o" hex=48c3a9ff006f`},

		// nested message decoded as length prefixed
		{"Timestamp", &protodemo.DecodeDemo{Timestamp: tsProto}, "", `len=12 str="............"`},
		// nested message decoded
		{"Timestamp", &protodemo.DecodeDemo{Timestamp: tsProto}, "4", `  bytes 2-8: field=1 type=0 (varint) uint=1607863096`},

		// any with specification
		{"Any", &protodemo.DecodeDemo{Any: tsAny}, "5.2", `bytes 49-55: field=1 type=0 (varint) uint=1607863096`},
	}

	decodeOutput := &bytes.Buffer{}
	for _, testCase := range testCases {
		decodeOutput.Reset()
		t.Run(testCase.name, func(t *testing.T) {
			serialized, err := proto.Marshal(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			nested, err := parseNested(testCase.nested)
			if err != nil {
				t.Fatal(err)
			}
			err = decode(decodeOutput, serialized, nested)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(decodeOutput.String(), testCase.expectedSubstr) {
				t.Errorf("failed to find %#v in output:\n%s",
					testCase.expectedSubstr, decodeOutput.String())
			}
		})
	}
}

func TestTruncated(t *testing.T) {
	// truncating a serialized message
	serialized, err := proto.Marshal(&protodemo.DecodeDemo{Int64Value: 42, StringValue: "str", BytesValue: []byte("bytes")})
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name           string
		offset         int
		expectedSubstr string
		expectedErr    string
	}{
		{"empty", 0, "", ""},
		{"InInt", 1, "", "decode failed at offset 1"},
		{"AfterInt", 2, "uint=42", ""},
		{"InStr", 3, "uint=42", "decode failed at offset 3"},
		{"AfterStr", 8, `str="str"`, ""},
	}

	decodeOutput := &bytes.Buffer{}
	for _, testCase := range testCases {
		decodeOutput.Reset()
		t.Run(testCase.name, func(t *testing.T) {
			err = decode(decodeOutput, serialized[:testCase.offset], nil)
			if err != nil {
				if !strings.Contains(err.Error(), testCase.expectedErr) {
					t.Errorf("failed to find %#v in err:\n%s",
						testCase.expectedErr, err.Error())
				}
			} else if testCase.expectedErr != "" {
				t.Errorf("err=nil; expected err to contain %#v", testCase.expectedErr)
			}

			if !strings.Contains(decodeOutput.String(), testCase.expectedSubstr) {
				t.Errorf("failed to find %#v in output:\n%s",
					testCase.expectedSubstr, decodeOutput.String())
			}
		})
	}
}
