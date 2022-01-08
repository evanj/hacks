// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.2
// source: protodecode/protodemo/demo.proto

package protodemo

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// DecodeDemo contains many types to test decoding.
// See: https://developers.google.com/protocol-buffers/docs/proto3#scalar
type DecodeDemo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Int64Value  int64                  `protobuf:"varint,1,opt,name=int64_value,json=int64Value,proto3" json:"int64_value,omitempty"`
	StringValue string                 `protobuf:"bytes,2,opt,name=string_value,json=stringValue,proto3" json:"string_value,omitempty"`
	BytesValue  []byte                 `protobuf:"bytes,3,opt,name=bytes_value,json=bytesValue,proto3" json:"bytes_value,omitempty"`
	Timestamp   *timestamppb.Timestamp `protobuf:"bytes,4,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Any         *anypb.Any             `protobuf:"bytes,5,opt,name=any,proto3" json:"any,omitempty"`
}

func (x *DecodeDemo) Reset() {
	*x = DecodeDemo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_protodecode_protodemo_demo_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DecodeDemo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DecodeDemo) ProtoMessage() {}

func (x *DecodeDemo) ProtoReflect() protoreflect.Message {
	mi := &file_protodecode_protodemo_demo_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DecodeDemo.ProtoReflect.Descriptor instead.
func (*DecodeDemo) Descriptor() ([]byte, []int) {
	return file_protodecode_protodemo_demo_proto_rawDescGZIP(), []int{0}
}

func (x *DecodeDemo) GetInt64Value() int64 {
	if x != nil {
		return x.Int64Value
	}
	return 0
}

func (x *DecodeDemo) GetStringValue() string {
	if x != nil {
		return x.StringValue
	}
	return ""
}

func (x *DecodeDemo) GetBytesValue() []byte {
	if x != nil {
		return x.BytesValue
	}
	return nil
}

func (x *DecodeDemo) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *DecodeDemo) GetAny() *anypb.Any {
	if x != nil {
		return x.Any
	}
	return nil
}

var File_protodecode_protodemo_demo_proto protoreflect.FileDescriptor

var file_protodecode_protodemo_demo_proto_rawDesc = []byte{
	0x0a, 0x20, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x64, 0x65, 0x6d, 0x6f, 0x2f, 0x64, 0x65, 0x6d, 0x6f, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x09, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x65, 0x6d, 0x6f, 0x1a, 0x1f, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x19,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd3, 0x01, 0x0a, 0x0a, 0x44, 0x65,
	0x63, 0x6f, 0x64, 0x65, 0x44, 0x65, 0x6d, 0x6f, 0x12, 0x1f, 0x0a, 0x0b, 0x69, 0x6e, 0x74, 0x36,
	0x34, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0a, 0x69,
	0x6e, 0x74, 0x36, 0x34, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x74, 0x72,
	0x69, 0x6e, 0x67, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0b, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x1f, 0x0a, 0x0b,
	0x62, 0x79, 0x74, 0x65, 0x73, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x0a, 0x62, 0x79, 0x74, 0x65, 0x73, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x38, 0x0a,
	0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x26, 0x0a, 0x03, 0x61, 0x6e, 0x79, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x03, 0x61, 0x6e, 0x79, 0x42,
	0x2e, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x65, 0x76,
	0x61, 0x6e, 0x6a, 0x2f, 0x68, 0x61, 0x63, 0x6b, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64,
	0x65, 0x63, 0x6f, 0x64, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x64, 0x65, 0x6d, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_protodecode_protodemo_demo_proto_rawDescOnce sync.Once
	file_protodecode_protodemo_demo_proto_rawDescData = file_protodecode_protodemo_demo_proto_rawDesc
)

func file_protodecode_protodemo_demo_proto_rawDescGZIP() []byte {
	file_protodecode_protodemo_demo_proto_rawDescOnce.Do(func() {
		file_protodecode_protodemo_demo_proto_rawDescData = protoimpl.X.CompressGZIP(file_protodecode_protodemo_demo_proto_rawDescData)
	})
	return file_protodecode_protodemo_demo_proto_rawDescData
}

var file_protodecode_protodemo_demo_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_protodecode_protodemo_demo_proto_goTypes = []interface{}{
	(*DecodeDemo)(nil),            // 0: protodemo.DecodeDemo
	(*timestamppb.Timestamp)(nil), // 1: google.protobuf.Timestamp
	(*anypb.Any)(nil),             // 2: google.protobuf.Any
}
var file_protodecode_protodemo_demo_proto_depIdxs = []int32{
	1, // 0: protodemo.DecodeDemo.timestamp:type_name -> google.protobuf.Timestamp
	2, // 1: protodemo.DecodeDemo.any:type_name -> google.protobuf.Any
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_protodecode_protodemo_demo_proto_init() }
func file_protodecode_protodemo_demo_proto_init() {
	if File_protodecode_protodemo_demo_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_protodecode_protodemo_demo_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DecodeDemo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_protodecode_protodemo_demo_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_protodecode_protodemo_demo_proto_goTypes,
		DependencyIndexes: file_protodecode_protodemo_demo_proto_depIdxs,
		MessageInfos:      file_protodecode_protodemo_demo_proto_msgTypes,
	}.Build()
	File_protodecode_protodemo_demo_proto = out.File
	file_protodecode_protodemo_demo_proto_rawDesc = nil
	file_protodecode_protodemo_demo_proto_goTypes = nil
	file_protodecode_protodemo_demo_proto_depIdxs = nil
}
