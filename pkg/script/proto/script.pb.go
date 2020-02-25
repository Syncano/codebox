// Code generated by protoc-gen-go. DO NOT EDIT.
// source: pkg/script/proto/script.proto

package proto

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// RunRequest represents either a Meta message or a Chunk message.
// It should always consist of exactly 1 Meta and optionally repeated Chunk messages.
type RunRequest struct {
	// Types that are valid to be assigned to Value:
	//	*RunRequest_Meta
	//	*RunRequest_Chunk
	Value                isRunRequest_Value `protobuf_oneof:"value"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *RunRequest) Reset()         { *m = RunRequest{} }
func (m *RunRequest) String() string { return proto.CompactTextString(m) }
func (*RunRequest) ProtoMessage()    {}
func (*RunRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_f4c2665a2612bf7e, []int{0}
}

func (m *RunRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunRequest.Unmarshal(m, b)
}
func (m *RunRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunRequest.Marshal(b, m, deterministic)
}
func (m *RunRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRequest.Merge(m, src)
}
func (m *RunRequest) XXX_Size() int {
	return xxx_messageInfo_RunRequest.Size(m)
}
func (m *RunRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RunRequest proto.InternalMessageInfo

type isRunRequest_Value interface {
	isRunRequest_Value()
}

type RunRequest_Meta struct {
	Meta *RunRequest_MetaMessage `protobuf:"bytes,1,opt,name=meta,proto3,oneof"`
}

type RunRequest_Chunk struct {
	Chunk *RunRequest_ChunkMessage `protobuf:"bytes,2,opt,name=chunk,proto3,oneof"`
}

func (*RunRequest_Meta) isRunRequest_Value() {}

func (*RunRequest_Chunk) isRunRequest_Value() {}

func (m *RunRequest) GetValue() isRunRequest_Value {
	if m != nil {
		return m.Value
	}
	return nil
}

func (m *RunRequest) GetMeta() *RunRequest_MetaMessage {
	if x, ok := m.GetValue().(*RunRequest_Meta); ok {
		return x.Meta
	}
	return nil
}

func (m *RunRequest) GetChunk() *RunRequest_ChunkMessage {
	if x, ok := m.GetValue().(*RunRequest_Chunk); ok {
		return x.Chunk
	}
	return nil
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*RunRequest) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*RunRequest_Meta)(nil),
		(*RunRequest_Chunk)(nil),
	}
}

// Meta message specifies fields to describe what is being run.
type RunRequest_MetaMessage struct {
	RequestID            string                                 `protobuf:"bytes,7,opt,name=requestID,proto3" json:"requestID,omitempty"`
	Runtime              string                                 `protobuf:"bytes,1,opt,name=runtime,proto3" json:"runtime,omitempty"`
	SourceHash           string                                 `protobuf:"bytes,2,opt,name=sourceHash,proto3" json:"sourceHash,omitempty"`
	UserID               string                                 `protobuf:"bytes,3,opt,name=userID,proto3" json:"userID,omitempty"`
	Options              *RunRequest_MetaMessage_OptionsMessage `protobuf:"bytes,5,opt,name=options,proto3" json:"options,omitempty"`
	Environment          string                                 `protobuf:"bytes,6,opt,name=environment,proto3" json:"environment,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                               `json:"-"`
	XXX_unrecognized     []byte                                 `json:"-"`
	XXX_sizecache        int32                                  `json:"-"`
}

func (m *RunRequest_MetaMessage) Reset()         { *m = RunRequest_MetaMessage{} }
func (m *RunRequest_MetaMessage) String() string { return proto.CompactTextString(m) }
func (*RunRequest_MetaMessage) ProtoMessage()    {}
func (*RunRequest_MetaMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f4c2665a2612bf7e, []int{0, 0}
}

func (m *RunRequest_MetaMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunRequest_MetaMessage.Unmarshal(m, b)
}
func (m *RunRequest_MetaMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunRequest_MetaMessage.Marshal(b, m, deterministic)
}
func (m *RunRequest_MetaMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRequest_MetaMessage.Merge(m, src)
}
func (m *RunRequest_MetaMessage) XXX_Size() int {
	return xxx_messageInfo_RunRequest_MetaMessage.Size(m)
}
func (m *RunRequest_MetaMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRequest_MetaMessage.DiscardUnknown(m)
}

var xxx_messageInfo_RunRequest_MetaMessage proto.InternalMessageInfo

func (m *RunRequest_MetaMessage) GetRequestID() string {
	if m != nil {
		return m.RequestID
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetRuntime() string {
	if m != nil {
		return m.Runtime
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetSourceHash() string {
	if m != nil {
		return m.SourceHash
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetUserID() string {
	if m != nil {
		return m.UserID
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetOptions() *RunRequest_MetaMessage_OptionsMessage {
	if m != nil {
		return m.Options
	}
	return nil
}

func (m *RunRequest_MetaMessage) GetEnvironment() string {
	if m != nil {
		return m.Environment
	}
	return ""
}

type RunRequest_MetaMessage_OptionsMessage struct {
	EntryPoint  string `protobuf:"bytes,1,opt,name=entryPoint,proto3" json:"entryPoint,omitempty"`
	OutputLimit uint32 `protobuf:"varint,2,opt,name=outputLimit,proto3" json:"outputLimit,omitempty"`
	Timeout     int64  `protobuf:"varint,3,opt,name=timeout,proto3" json:"timeout,omitempty"`
	MCPU        uint32 `protobuf:"varint,7,opt,name=mCPU,proto3" json:"mCPU,omitempty"`
	Async       uint32 `protobuf:"varint,8,opt,name=async,proto3" json:"async,omitempty"`
	// Empty args, config, meta are acceptable.
	Args                 []byte   `protobuf:"bytes,4,opt,name=args,proto3" json:"args,omitempty"`
	Config               []byte   `protobuf:"bytes,5,opt,name=config,proto3" json:"config,omitempty"`
	Meta                 []byte   `protobuf:"bytes,6,opt,name=meta,proto3" json:"meta,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RunRequest_MetaMessage_OptionsMessage) Reset()         { *m = RunRequest_MetaMessage_OptionsMessage{} }
func (m *RunRequest_MetaMessage_OptionsMessage) String() string { return proto.CompactTextString(m) }
func (*RunRequest_MetaMessage_OptionsMessage) ProtoMessage()    {}
func (*RunRequest_MetaMessage_OptionsMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f4c2665a2612bf7e, []int{0, 0, 0}
}

func (m *RunRequest_MetaMessage_OptionsMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunRequest_MetaMessage_OptionsMessage.Unmarshal(m, b)
}
func (m *RunRequest_MetaMessage_OptionsMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunRequest_MetaMessage_OptionsMessage.Marshal(b, m, deterministic)
}
func (m *RunRequest_MetaMessage_OptionsMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRequest_MetaMessage_OptionsMessage.Merge(m, src)
}
func (m *RunRequest_MetaMessage_OptionsMessage) XXX_Size() int {
	return xxx_messageInfo_RunRequest_MetaMessage_OptionsMessage.Size(m)
}
func (m *RunRequest_MetaMessage_OptionsMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRequest_MetaMessage_OptionsMessage.DiscardUnknown(m)
}

var xxx_messageInfo_RunRequest_MetaMessage_OptionsMessage proto.InternalMessageInfo

func (m *RunRequest_MetaMessage_OptionsMessage) GetEntryPoint() string {
	if m != nil {
		return m.EntryPoint
	}
	return ""
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetOutputLimit() uint32 {
	if m != nil {
		return m.OutputLimit
	}
	return 0
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetTimeout() int64 {
	if m != nil {
		return m.Timeout
	}
	return 0
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetMCPU() uint32 {
	if m != nil {
		return m.MCPU
	}
	return 0
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetAsync() uint32 {
	if m != nil {
		return m.Async
	}
	return 0
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetArgs() []byte {
	if m != nil {
		return m.Args
	}
	return nil
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetConfig() []byte {
	if m != nil {
		return m.Config
	}
	return nil
}

func (m *RunRequest_MetaMessage_OptionsMessage) GetMeta() []byte {
	if m != nil {
		return m.Meta
	}
	return nil
}

type RunRequest_ChunkMessage struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Filename             string   `protobuf:"bytes,2,opt,name=filename,proto3" json:"filename,omitempty"`
	ContentType          string   `protobuf:"bytes,3,opt,name=contentType,proto3" json:"contentType,omitempty"`
	Data                 []byte   `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RunRequest_ChunkMessage) Reset()         { *m = RunRequest_ChunkMessage{} }
func (m *RunRequest_ChunkMessage) String() string { return proto.CompactTextString(m) }
func (*RunRequest_ChunkMessage) ProtoMessage()    {}
func (*RunRequest_ChunkMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f4c2665a2612bf7e, []int{0, 1}
}

func (m *RunRequest_ChunkMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunRequest_ChunkMessage.Unmarshal(m, b)
}
func (m *RunRequest_ChunkMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunRequest_ChunkMessage.Marshal(b, m, deterministic)
}
func (m *RunRequest_ChunkMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRequest_ChunkMessage.Merge(m, src)
}
func (m *RunRequest_ChunkMessage) XXX_Size() int {
	return xxx_messageInfo_RunRequest_ChunkMessage.Size(m)
}
func (m *RunRequest_ChunkMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRequest_ChunkMessage.DiscardUnknown(m)
}

var xxx_messageInfo_RunRequest_ChunkMessage proto.InternalMessageInfo

func (m *RunRequest_ChunkMessage) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *RunRequest_ChunkMessage) GetFilename() string {
	if m != nil {
		return m.Filename
	}
	return ""
}

func (m *RunRequest_ChunkMessage) GetContentType() string {
	if m != nil {
		return m.ContentType
	}
	return ""
}

func (m *RunRequest_ChunkMessage) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

// HTTPResponseMessage describes custom response from script.
type HTTPResponseMessage struct {
	StatusCode           int32             `protobuf:"varint,1,opt,name=statusCode,proto3" json:"statusCode,omitempty"`
	ContentType          string            `protobuf:"bytes,2,opt,name=contentType,proto3" json:"contentType,omitempty"`
	Content              []byte            `protobuf:"bytes,3,opt,name=content,proto3" json:"content,omitempty"`
	Headers              map[string]string `protobuf:"bytes,4,rep,name=headers,proto3" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *HTTPResponseMessage) Reset()         { *m = HTTPResponseMessage{} }
func (m *HTTPResponseMessage) String() string { return proto.CompactTextString(m) }
func (*HTTPResponseMessage) ProtoMessage()    {}
func (*HTTPResponseMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_f4c2665a2612bf7e, []int{1}
}

func (m *HTTPResponseMessage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HTTPResponseMessage.Unmarshal(m, b)
}
func (m *HTTPResponseMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HTTPResponseMessage.Marshal(b, m, deterministic)
}
func (m *HTTPResponseMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HTTPResponseMessage.Merge(m, src)
}
func (m *HTTPResponseMessage) XXX_Size() int {
	return xxx_messageInfo_HTTPResponseMessage.Size(m)
}
func (m *HTTPResponseMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_HTTPResponseMessage.DiscardUnknown(m)
}

var xxx_messageInfo_HTTPResponseMessage proto.InternalMessageInfo

func (m *HTTPResponseMessage) GetStatusCode() int32 {
	if m != nil {
		return m.StatusCode
	}
	return 0
}

func (m *HTTPResponseMessage) GetContentType() string {
	if m != nil {
		return m.ContentType
	}
	return ""
}

func (m *HTTPResponseMessage) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

func (m *HTTPResponseMessage) GetHeaders() map[string]string {
	if m != nil {
		return m.Headers
	}
	return nil
}

type RunResponse struct {
	ContainerID          string               `protobuf:"bytes,9,opt,name=containerID,proto3" json:"containerID,omitempty"`
	Code                 int32                `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Stdout               []byte               `protobuf:"bytes,2,opt,name=stdout,proto3" json:"stdout,omitempty"`
	Stderr               []byte               `protobuf:"bytes,3,opt,name=stderr,proto3" json:"stderr,omitempty"`
	Response             *HTTPResponseMessage `protobuf:"bytes,4,opt,name=response,proto3" json:"response,omitempty"`
	Took                 int64                `protobuf:"varint,5,opt,name=took,proto3" json:"took,omitempty"`
	Cached               bool                 `protobuf:"varint,6,opt,name=cached,proto3" json:"cached,omitempty"`
	Time                 int64                `protobuf:"varint,7,opt,name=time,proto3" json:"time,omitempty"`
	Weight               uint32               `protobuf:"varint,8,opt,name=weight,proto3" json:"weight,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *RunResponse) Reset()         { *m = RunResponse{} }
func (m *RunResponse) String() string { return proto.CompactTextString(m) }
func (*RunResponse) ProtoMessage()    {}
func (*RunResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_f4c2665a2612bf7e, []int{2}
}

func (m *RunResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunResponse.Unmarshal(m, b)
}
func (m *RunResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunResponse.Marshal(b, m, deterministic)
}
func (m *RunResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunResponse.Merge(m, src)
}
func (m *RunResponse) XXX_Size() int {
	return xxx_messageInfo_RunResponse.Size(m)
}
func (m *RunResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_RunResponse.DiscardUnknown(m)
}

var xxx_messageInfo_RunResponse proto.InternalMessageInfo

func (m *RunResponse) GetContainerID() string {
	if m != nil {
		return m.ContainerID
	}
	return ""
}

func (m *RunResponse) GetCode() int32 {
	if m != nil {
		return m.Code
	}
	return 0
}

func (m *RunResponse) GetStdout() []byte {
	if m != nil {
		return m.Stdout
	}
	return nil
}

func (m *RunResponse) GetStderr() []byte {
	if m != nil {
		return m.Stderr
	}
	return nil
}

func (m *RunResponse) GetResponse() *HTTPResponseMessage {
	if m != nil {
		return m.Response
	}
	return nil
}

func (m *RunResponse) GetTook() int64 {
	if m != nil {
		return m.Took
	}
	return 0
}

func (m *RunResponse) GetCached() bool {
	if m != nil {
		return m.Cached
	}
	return false
}

func (m *RunResponse) GetTime() int64 {
	if m != nil {
		return m.Time
	}
	return 0
}

func (m *RunResponse) GetWeight() uint32 {
	if m != nil {
		return m.Weight
	}
	return 0
}

func init() {
	proto.RegisterType((*RunRequest)(nil), "script.RunRequest")
	proto.RegisterType((*RunRequest_MetaMessage)(nil), "script.RunRequest.MetaMessage")
	proto.RegisterType((*RunRequest_MetaMessage_OptionsMessage)(nil), "script.RunRequest.MetaMessage.OptionsMessage")
	proto.RegisterType((*RunRequest_ChunkMessage)(nil), "script.RunRequest.ChunkMessage")
	proto.RegisterType((*HTTPResponseMessage)(nil), "script.HTTPResponseMessage")
	proto.RegisterMapType((map[string]string)(nil), "script.HTTPResponseMessage.HeadersEntry")
	proto.RegisterType((*RunResponse)(nil), "script.RunResponse")
}

func init() { proto.RegisterFile("pkg/script/proto/script.proto", fileDescriptor_f4c2665a2612bf7e) }

var fileDescriptor_f4c2665a2612bf7e = []byte{
	// 662 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x54, 0xcd, 0x6e, 0x13, 0x31,
	0x10, 0x66, 0xb3, 0x4d, 0x36, 0x99, 0xa4, 0x08, 0xb9, 0xa8, 0x5a, 0x05, 0x28, 0x51, 0x4f, 0x91,
	0x50, 0x13, 0x14, 0x90, 0x8a, 0x7a, 0x4c, 0x8b, 0x48, 0x25, 0x2a, 0x2a, 0xb7, 0x5c, 0xb8, 0xb9,
	0x1b, 0x37, 0x59, 0xa5, 0xb1, 0x17, 0xff, 0x14, 0x72, 0xe7, 0xb9, 0x90, 0x78, 0x09, 0xde, 0x83,
	0x37, 0x40, 0x1e, 0x7b, 0x9b, 0xed, 0x8f, 0x7a, 0xda, 0xf9, 0x3e, 0xcf, 0x78, 0xe6, 0x9b, 0x19,
	0x2f, 0xbc, 0x2a, 0x16, 0xb3, 0xa1, 0xce, 0x54, 0x5e, 0x98, 0x61, 0xa1, 0xa4, 0x91, 0x01, 0x0c,
	0x10, 0x90, 0x86, 0x47, 0xbb, 0xbf, 0xeb, 0x00, 0xd4, 0x0a, 0xca, 0xbf, 0x5b, 0xae, 0x0d, 0x79,
	0x0f, 0x1b, 0x4b, 0x6e, 0x58, 0x1a, 0xf5, 0xa2, 0x7e, 0x7b, 0xb4, 0x33, 0x08, 0x31, 0x6b, 0x8f,
	0xc1, 0x09, 0x37, 0xec, 0x84, 0x6b, 0xcd, 0x66, 0x7c, 0xf2, 0x84, 0xa2, 0x37, 0xd9, 0x87, 0x7a,
	0x36, 0xb7, 0x62, 0x91, 0xd6, 0x30, 0xec, 0xf5, 0x03, 0x61, 0x87, 0xee, 0x7c, 0x1d, 0xe7, 0xfd,
	0xbb, 0x7f, 0x62, 0x68, 0x57, 0x2e, 0x24, 0x2f, 0xa1, 0xa5, 0x7c, 0xc0, 0xf1, 0x51, 0x9a, 0xf4,
	0xa2, 0x7e, 0x8b, 0xae, 0x09, 0x92, 0x42, 0xa2, 0xac, 0x30, 0xf9, 0x92, 0x63, 0x7d, 0x2d, 0x5a,
	0x42, 0xb2, 0x03, 0xa0, 0xa5, 0x55, 0x19, 0x9f, 0x30, 0x3d, 0xc7, 0x2a, 0x5a, 0xb4, 0xc2, 0x90,
	0x6d, 0x68, 0x58, 0xcd, 0xd5, 0xf1, 0x51, 0x1a, 0xe3, 0x59, 0x40, 0xe4, 0x13, 0x24, 0xb2, 0x30,
	0xb9, 0x14, 0x3a, 0xad, 0x63, 0xe9, 0x7b, 0x8f, 0x2b, 0x1e, 0x7c, 0xf1, 0xde, 0x01, 0xd2, 0x32,
	0x9a, 0xf4, 0xa0, 0xcd, 0xc5, 0x75, 0xae, 0xa4, 0x58, 0x72, 0x61, 0xd2, 0x06, 0x66, 0xa9, 0x52,
	0xdd, 0xbf, 0x11, 0x3c, 0xbd, 0x1d, 0xed, 0xaa, 0xe6, 0xc2, 0xa8, 0xd5, 0xa9, 0xcc, 0x85, 0x09,
	0x92, 0x2a, 0x8c, 0xbb, 0x54, 0x5a, 0x53, 0x58, 0xf3, 0x39, 0x5f, 0xe6, 0x06, 0x65, 0x6d, 0xd2,
	0x2a, 0xe5, 0x3a, 0xe2, 0xf4, 0x4b, 0x6b, 0x50, 0x58, 0x4c, 0x4b, 0x48, 0x08, 0x6c, 0x2c, 0x0f,
	0x4f, 0xbf, 0x62, 0x13, 0x37, 0x29, 0xda, 0xe4, 0x39, 0xd4, 0x99, 0x5e, 0x89, 0x2c, 0x6d, 0x22,
	0xe9, 0x81, 0xf3, 0x64, 0x6a, 0xa6, 0xd3, 0x8d, 0x5e, 0xd4, 0xef, 0x50, 0xb4, 0x5d, 0xbf, 0x32,
	0x29, 0x2e, 0xf3, 0x19, 0xb6, 0xa5, 0x43, 0x03, 0xc2, 0x5b, 0xdd, 0x7a, 0x34, 0xbc, 0xaf, 0xb3,
	0xbb, 0x06, 0x3a, 0xd5, 0xe1, 0x3a, 0x1f, 0xc1, 0x6e, 0x46, 0x84, 0x36, 0xe9, 0x42, 0xf3, 0x32,
	0xbf, 0xe2, 0xc8, 0xfb, 0xe9, 0xdc, 0x60, 0xa7, 0x32, 0x93, 0xc2, 0x70, 0x61, 0xce, 0x57, 0x05,
	0x0f, 0x03, 0xaa, 0x52, 0xee, 0xc6, 0x29, 0x33, 0xac, 0xac, 0xd0, 0xd9, 0xe3, 0x04, 0xea, 0xd7,
	0xec, 0xca, 0xf2, 0xdd, 0x7f, 0x11, 0x6c, 0x4d, 0xce, 0xcf, 0x4f, 0x29, 0xd7, 0x85, 0x14, 0x9a,
	0x57, 0x9a, 0xab, 0x0d, 0x33, 0x56, 0x1f, 0xca, 0xa9, 0x2f, 0xa6, 0x4e, 0x2b, 0xcc, 0xdd, 0xb4,
	0xb5, 0xfb, 0x69, 0x53, 0x48, 0x02, 0xc4, 0xa2, 0x3a, 0xb4, 0x84, 0x64, 0x0c, 0xc9, 0x9c, 0xb3,
	0x29, 0x57, 0xae, 0x6b, 0x71, 0xbf, 0x3d, 0xea, 0x97, 0x6b, 0xf3, 0x40, 0x25, 0x83, 0x89, 0x77,
	0xfd, 0xe8, 0x06, 0x4b, 0xcb, 0xc0, 0xee, 0x01, 0x74, 0xaa, 0x07, 0xe4, 0x19, 0xc4, 0x0b, 0xbe,
	0x0a, 0x5d, 0x73, 0xa6, 0x1b, 0x17, 0x4a, 0x0c, 0xb5, 0x79, 0x70, 0x50, 0xfb, 0x10, 0xed, 0xfe,
	0xaa, 0x41, 0x1b, 0x17, 0xd4, 0x27, 0x2a, 0xb5, 0xb0, 0x5c, 0xe0, 0x8e, 0xb7, 0xd6, 0x5a, 0x02,
	0xe5, 0x5a, 0x98, 0xad, 0xfb, 0x80, 0xb6, 0x1b, 0xb2, 0x36, 0x53, 0xb7, 0x3b, 0x35, 0x3f, 0x64,
	0x8f, 0x02, 0xcf, 0x95, 0x0a, 0xb2, 0x03, 0x22, 0xfb, 0xd0, 0x54, 0x21, 0x23, 0x8e, 0xa2, 0x3d,
	0x7a, 0xf1, 0x88, 0x6c, 0x7a, 0xe3, 0xec, 0x92, 0x1b, 0x29, 0x17, 0xb8, 0x4b, 0x31, 0x45, 0x1b,
	0x37, 0x8c, 0x65, 0x73, 0x3e, 0xc5, 0x5d, 0x6a, 0xd2, 0x80, 0xd0, 0xd7, 0x3d, 0xf0, 0x24, 0xf8,
	0xba, 0xd7, 0xbd, 0x0d, 0x8d, 0x1f, 0x3c, 0x9f, 0xcd, 0x4d, 0x58, 0xdc, 0x80, 0x46, 0x63, 0xe8,
	0x9c, 0x61, 0x7e, 0x6a, 0x85, 0xe0, 0x8a, 0x8c, 0x20, 0xa6, 0x56, 0x10, 0x72, 0xff, 0x0d, 0x77,
	0xb7, 0x6e, 0x71, 0xbe, 0xae, 0x7e, 0xf4, 0x36, 0x1a, 0xef, 0x7d, 0x7b, 0x33, 0xcb, 0xcd, 0xdc,
	0x5e, 0x0c, 0x32, 0xb9, 0x1c, 0x9e, 0xad, 0x44, 0xc6, 0x84, 0x1c, 0xba, 0xfe, 0x5c, 0xc8, 0x9f,
	0xc3, 0xbb, 0xff, 0xd0, 0x8b, 0x06, 0x7e, 0xde, 0xfd, 0x0f, 0x00, 0x00, 0xff, 0xff, 0xc0, 0xd2,
	0x55, 0x79, 0x5e, 0x05, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ScriptRunnerClient is the client API for ScriptRunner service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ScriptRunnerClient interface {
	// Run runs script in secure environment of worker.
	Run(ctx context.Context, opts ...grpc.CallOption) (ScriptRunner_RunClient, error)
}

type scriptRunnerClient struct {
	cc *grpc.ClientConn
}

func NewScriptRunnerClient(cc *grpc.ClientConn) ScriptRunnerClient {
	return &scriptRunnerClient{cc}
}

func (c *scriptRunnerClient) Run(ctx context.Context, opts ...grpc.CallOption) (ScriptRunner_RunClient, error) {
	stream, err := c.cc.NewStream(ctx, &_ScriptRunner_serviceDesc.Streams[0], "/script.ScriptRunner/Run", opts...)
	if err != nil {
		return nil, err
	}
	x := &scriptRunnerRunClient{stream}
	return x, nil
}

type ScriptRunner_RunClient interface {
	Send(*RunRequest) error
	Recv() (*RunResponse, error)
	grpc.ClientStream
}

type scriptRunnerRunClient struct {
	grpc.ClientStream
}

func (x *scriptRunnerRunClient) Send(m *RunRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *scriptRunnerRunClient) Recv() (*RunResponse, error) {
	m := new(RunResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ScriptRunnerServer is the server API for ScriptRunner service.
type ScriptRunnerServer interface {
	// Run runs script in secure environment of worker.
	Run(ScriptRunner_RunServer) error
}

func RegisterScriptRunnerServer(s *grpc.Server, srv ScriptRunnerServer) {
	s.RegisterService(&_ScriptRunner_serviceDesc, srv)
}

func _ScriptRunner_Run_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ScriptRunnerServer).Run(&scriptRunnerRunServer{stream})
}

type ScriptRunner_RunServer interface {
	Send(*RunResponse) error
	Recv() (*RunRequest, error)
	grpc.ServerStream
}

type scriptRunnerRunServer struct {
	grpc.ServerStream
}

func (x *scriptRunnerRunServer) Send(m *RunResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *scriptRunnerRunServer) Recv() (*RunRequest, error) {
	m := new(RunRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _ScriptRunner_serviceDesc = grpc.ServiceDesc{
	ServiceName: "script.ScriptRunner",
	HandlerType: (*ScriptRunnerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Run",
			Handler:       _ScriptRunner_Run_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "pkg/script/proto/script.proto",
}
