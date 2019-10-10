// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: pkg/broker/proto/broker.proto

package proto

import (
	context "context"
	fmt "fmt"
	proto1 "github.com/Syncano/codebox/pkg/lb/proto"
	proto2 "github.com/Syncano/codebox/pkg/script/proto"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	io "io"
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
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// RunRequest represents either a Meta message or a Chunk message.
// It should always consist of exactly 1 Meta and optionally repeated Chunk messages.
type RunRequest struct {
	Meta                 *RunRequest_MetaMessage        `protobuf:"bytes,1,opt,name=meta,proto3" json:"meta,omitempty"`
	LbMeta               *proto1.RunRequest_MetaMessage `protobuf:"bytes,2,opt,name=lbMeta,proto3" json:"lbMeta,omitempty"`
	Request              []*proto2.RunRequest           `protobuf:"bytes,3,rep,name=request,proto3" json:"request,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                       `json:"-"`
	XXX_unrecognized     []byte                         `json:"-"`
	XXX_sizecache        int32                          `json:"-"`
}

func (m *RunRequest) Reset()         { *m = RunRequest{} }
func (m *RunRequest) String() string { return proto.CompactTextString(m) }
func (*RunRequest) ProtoMessage()    {}
func (*RunRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_6323966a5e0dffec, []int{0}
}
func (m *RunRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RunRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_RunRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *RunRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRequest.Merge(m, src)
}
func (m *RunRequest) XXX_Size() int {
	return m.Size()
}
func (m *RunRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RunRequest proto.InternalMessageInfo

func (m *RunRequest) GetMeta() *RunRequest_MetaMessage {
	if m != nil {
		return m.Meta
	}
	return nil
}

func (m *RunRequest) GetLbMeta() *proto1.RunRequest_MetaMessage {
	if m != nil {
		return m.LbMeta
	}
	return nil
}

func (m *RunRequest) GetRequest() []*proto2.RunRequest {
	if m != nil {
		return m.Request
	}
	return nil
}

// Meta message specifies fields to describe what is being run.
type RunRequest_MetaMessage struct {
	Files                map[string]string `protobuf:"bytes,1,rep,name=files,proto3" json:"files,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	EnvironmentURL       string            `protobuf:"bytes,2,opt,name=environmentURL,proto3" json:"environmentURL,omitempty"`
	Trace                []byte            `protobuf:"bytes,3,opt,name=trace,proto3" json:"trace,omitempty"`
	TraceID              uint64            `protobuf:"varint,4,opt,name=traceID,proto3" json:"traceID,omitempty"`
	Sync                 bool              `protobuf:"varint,5,opt,name=sync,proto3" json:"sync,omitempty"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *RunRequest_MetaMessage) Reset()         { *m = RunRequest_MetaMessage{} }
func (m *RunRequest_MetaMessage) String() string { return proto.CompactTextString(m) }
func (*RunRequest_MetaMessage) ProtoMessage()    {}
func (*RunRequest_MetaMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_6323966a5e0dffec, []int{0, 0}
}
func (m *RunRequest_MetaMessage) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RunRequest_MetaMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_RunRequest_MetaMessage.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *RunRequest_MetaMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRequest_MetaMessage.Merge(m, src)
}
func (m *RunRequest_MetaMessage) XXX_Size() int {
	return m.Size()
}
func (m *RunRequest_MetaMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRequest_MetaMessage.DiscardUnknown(m)
}

var xxx_messageInfo_RunRequest_MetaMessage proto.InternalMessageInfo

func (m *RunRequest_MetaMessage) GetFiles() map[string]string {
	if m != nil {
		return m.Files
	}
	return nil
}

func (m *RunRequest_MetaMessage) GetEnvironmentURL() string {
	if m != nil {
		return m.EnvironmentURL
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetTrace() []byte {
	if m != nil {
		return m.Trace
	}
	return nil
}

func (m *RunRequest_MetaMessage) GetTraceID() uint64 {
	if m != nil {
		return m.TraceID
	}
	return 0
}

func (m *RunRequest_MetaMessage) GetSync() bool {
	if m != nil {
		return m.Sync
	}
	return false
}

func init() {
	proto.RegisterType((*RunRequest)(nil), "broker.RunRequest")
	proto.RegisterType((*RunRequest_MetaMessage)(nil), "broker.RunRequest.MetaMessage")
	proto.RegisterMapType((map[string]string)(nil), "broker.RunRequest.MetaMessage.FilesEntry")
}

func init() { proto.RegisterFile("pkg/broker/proto/broker.proto", fileDescriptor_6323966a5e0dffec) }

var fileDescriptor_6323966a5e0dffec = []byte{
	// 394 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x92, 0xd1, 0x6a, 0xdb, 0x30,
	0x14, 0x86, 0xa7, 0xd8, 0x49, 0x96, 0x93, 0x30, 0x86, 0xb6, 0x0b, 0xcd, 0x30, 0x63, 0x76, 0x31,
	0x3c, 0x36, 0xec, 0xe0, 0xdd, 0x84, 0xc1, 0xd8, 0x18, 0x6b, 0xa1, 0xd0, 0xdc, 0x28, 0xf4, 0xa6,
	0x77, 0xb6, 0xab, 0xa6, 0x26, 0x8e, 0xe4, 0x4a, 0x72, 0xa8, 0xdf, 0xa4, 0x7d, 0xa3, 0x5e, 0xf6,
	0x11, 0x4a, 0xfa, 0x0a, 0x7d, 0x80, 0x62, 0xd9, 0x21, 0x6d, 0x4a, 0x7b, 0xe5, 0xf3, 0xeb, 0x7c,
	0xbf, 0xce, 0xef, 0x63, 0xc3, 0xe7, 0x62, 0x31, 0x0f, 0x13, 0x29, 0x16, 0x4c, 0x86, 0x85, 0x14,
	0x5a, 0xb4, 0x22, 0x30, 0x02, 0xf7, 0x1a, 0xe5, 0x7c, 0xaa, 0xb1, 0x3c, 0x69, 0x11, 0x59, 0x72,
	0xbe, 0x41, 0x1c, 0x73, 0x83, 0x4a, 0x65, 0x56, 0xe8, 0xb6, 0xdd, 0x88, 0xa6, 0xfd, 0xe5, 0xca,
	0x02, 0xa0, 0x25, 0xa7, 0xec, 0xbc, 0x64, 0x4a, 0xe3, 0x08, 0xec, 0x25, 0xd3, 0x31, 0x41, 0x1e,
	0xf2, 0x87, 0x91, 0x1b, 0xb4, 0xd3, 0xb6, 0x44, 0x30, 0x65, 0x3a, 0x9e, 0x32, 0xa5, 0xe2, 0x39,
	0xa3, 0x86, 0xc5, 0x11, 0xf4, 0xf2, 0xa4, 0x3e, 0x26, 0x1d, 0xe3, 0x72, 0x82, 0x3c, 0x79, 0xc9,
	0xd1, 0x92, 0xf8, 0x07, 0xf4, 0x65, 0xd3, 0x26, 0x96, 0x67, 0xf9, 0xc3, 0x08, 0x07, 0x6d, 0xac,
	0xad, 0x91, 0x6e, 0x10, 0xe7, 0x1e, 0xc1, 0xf0, 0xd1, 0x2d, 0xf8, 0x0f, 0x74, 0x4f, 0xb3, 0x9c,
	0x29, 0x82, 0x8c, 0xf7, 0xdb, 0xeb, 0x31, 0x83, 0xfd, 0x9a, 0xdd, 0xe3, 0x5a, 0x56, 0xb4, 0xf1,
	0xe1, 0xaf, 0xf0, 0x8e, 0xf1, 0x55, 0x26, 0x05, 0x5f, 0x32, 0xae, 0x8f, 0xe8, 0xa1, 0x89, 0x3e,
	0xa0, 0x3b, 0xa7, 0xf8, 0x23, 0x74, 0xb5, 0x8c, 0x53, 0x46, 0x2c, 0x0f, 0xf9, 0x23, 0xda, 0x08,
	0x4c, 0xa0, 0x6f, 0x8a, 0x83, 0xff, 0xc4, 0xf6, 0x90, 0x6f, 0xd3, 0x8d, 0xc4, 0x18, 0x6c, 0x55,
	0xf1, 0x94, 0x74, 0x3d, 0xe4, 0xbf, 0xa5, 0xa6, 0x76, 0x26, 0x00, 0xdb, 0x00, 0xf8, 0x3d, 0x58,
	0x0b, 0x56, 0x99, 0xfd, 0x0e, 0x68, 0x5d, 0xd6, 0x33, 0x56, 0x71, 0x5e, 0xb2, 0x36, 0x42, 0x23,
	0x7e, 0x75, 0x26, 0x28, 0xfa, 0x0b, 0xa3, 0x99, 0x59, 0x0a, 0x35, 0x1f, 0x14, 0x8f, 0xc1, 0xa2,
	0x25, 0xc7, 0xf8, 0xf9, 0xeb, 0x3a, 0x1f, 0x9e, 0xac, 0x4f, 0x15, 0x82, 0x2b, 0x36, 0x46, 0xff,
	0x7e, 0x5f, 0xaf, 0x5d, 0x74, 0xb3, 0x76, 0xd1, 0xed, 0xda, 0x45, 0x97, 0x77, 0xee, 0x9b, 0xe3,
	0xef, 0xf3, 0x4c, 0x9f, 0x95, 0x49, 0x90, 0x8a, 0x65, 0x38, 0xab, 0x78, 0x1a, 0x73, 0x11, 0xa6,
	0xe2, 0x84, 0x25, 0xe2, 0x22, 0xdc, 0xfd, 0xd7, 0x92, 0x9e, 0x79, 0xfc, 0x7c, 0x08, 0x00, 0x00,
	0xff, 0xff, 0x78, 0x77, 0x89, 0xbb, 0x86, 0x02, 0x00, 0x00,
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
	// Run runs script in secure environment.
	Run(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (ScriptRunner_RunClient, error)
}

type scriptRunnerClient struct {
	cc *grpc.ClientConn
}

func NewScriptRunnerClient(cc *grpc.ClientConn) ScriptRunnerClient {
	return &scriptRunnerClient{cc}
}

func (c *scriptRunnerClient) Run(ctx context.Context, in *RunRequest, opts ...grpc.CallOption) (ScriptRunner_RunClient, error) {
	stream, err := c.cc.NewStream(ctx, &_ScriptRunner_serviceDesc.Streams[0], "/broker.ScriptRunner/Run", opts...)
	if err != nil {
		return nil, err
	}
	x := &scriptRunnerRunClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type ScriptRunner_RunClient interface {
	Recv() (*proto2.RunResponse, error)
	grpc.ClientStream
}

type scriptRunnerRunClient struct {
	grpc.ClientStream
}

func (x *scriptRunnerRunClient) Recv() (*proto2.RunResponse, error) {
	m := new(proto2.RunResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ScriptRunnerServer is the server API for ScriptRunner service.
type ScriptRunnerServer interface {
	// Run runs script in secure environment.
	Run(*RunRequest, ScriptRunner_RunServer) error
}

func RegisterScriptRunnerServer(s *grpc.Server, srv ScriptRunnerServer) {
	s.RegisterService(&_ScriptRunner_serviceDesc, srv)
}

func _ScriptRunner_Run_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(RunRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ScriptRunnerServer).Run(m, &scriptRunnerRunServer{stream})
}

type ScriptRunner_RunServer interface {
	Send(*proto2.RunResponse) error
	grpc.ServerStream
}

type scriptRunnerRunServer struct {
	grpc.ServerStream
}

func (x *scriptRunnerRunServer) Send(m *proto2.RunResponse) error {
	return x.ServerStream.SendMsg(m)
}

var _ScriptRunner_serviceDesc = grpc.ServiceDesc{
	ServiceName: "broker.ScriptRunner",
	HandlerType: (*ScriptRunnerServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Run",
			Handler:       _ScriptRunner_Run_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "pkg/broker/proto/broker.proto",
}

func (m *RunRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RunRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Meta != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintBroker(dAtA, i, uint64(m.Meta.Size()))
		n1, err := m.Meta.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.LbMeta != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintBroker(dAtA, i, uint64(m.LbMeta.Size()))
		n2, err := m.LbMeta.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	if len(m.Request) > 0 {
		for _, msg := range m.Request {
			dAtA[i] = 0x1a
			i++
			i = encodeVarintBroker(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func (m *RunRequest_MetaMessage) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RunRequest_MetaMessage) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Files) > 0 {
		for k, _ := range m.Files {
			dAtA[i] = 0xa
			i++
			v := m.Files[k]
			mapSize := 1 + len(k) + sovBroker(uint64(len(k))) + 1 + len(v) + sovBroker(uint64(len(v)))
			i = encodeVarintBroker(dAtA, i, uint64(mapSize))
			dAtA[i] = 0xa
			i++
			i = encodeVarintBroker(dAtA, i, uint64(len(k)))
			i += copy(dAtA[i:], k)
			dAtA[i] = 0x12
			i++
			i = encodeVarintBroker(dAtA, i, uint64(len(v)))
			i += copy(dAtA[i:], v)
		}
	}
	if len(m.EnvironmentURL) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintBroker(dAtA, i, uint64(len(m.EnvironmentURL)))
		i += copy(dAtA[i:], m.EnvironmentURL)
	}
	if len(m.Trace) > 0 {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintBroker(dAtA, i, uint64(len(m.Trace)))
		i += copy(dAtA[i:], m.Trace)
	}
	if m.TraceID != 0 {
		dAtA[i] = 0x20
		i++
		i = encodeVarintBroker(dAtA, i, uint64(m.TraceID))
	}
	if m.Sync {
		dAtA[i] = 0x28
		i++
		if m.Sync {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i++
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeVarintBroker(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *RunRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Meta != nil {
		l = m.Meta.Size()
		n += 1 + l + sovBroker(uint64(l))
	}
	if m.LbMeta != nil {
		l = m.LbMeta.Size()
		n += 1 + l + sovBroker(uint64(l))
	}
	if len(m.Request) > 0 {
		for _, e := range m.Request {
			l = e.Size()
			n += 1 + l + sovBroker(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *RunRequest_MetaMessage) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Files) > 0 {
		for k, v := range m.Files {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + sovBroker(uint64(len(k))) + 1 + len(v) + sovBroker(uint64(len(v)))
			n += mapEntrySize + 1 + sovBroker(uint64(mapEntrySize))
		}
	}
	l = len(m.EnvironmentURL)
	if l > 0 {
		n += 1 + l + sovBroker(uint64(l))
	}
	l = len(m.Trace)
	if l > 0 {
		n += 1 + l + sovBroker(uint64(l))
	}
	if m.TraceID != 0 {
		n += 1 + sovBroker(uint64(m.TraceID))
	}
	if m.Sync {
		n += 2
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovBroker(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozBroker(x uint64) (n int) {
	return sovBroker(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *RunRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowBroker
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: RunRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RunRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Meta", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBroker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBroker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Meta == nil {
				m.Meta = &RunRequest_MetaMessage{}
			}
			if err := m.Meta.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field LbMeta", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBroker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBroker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.LbMeta == nil {
				m.LbMeta = &proto1.RunRequest_MetaMessage{}
			}
			if err := m.LbMeta.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Request", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBroker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBroker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Request = append(m.Request, &proto2.RunRequest{})
			if err := m.Request[len(m.Request)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipBroker(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthBroker
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthBroker
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *RunRequest_MetaMessage) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowBroker
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MetaMessage: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MetaMessage: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Files", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthBroker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthBroker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Files == nil {
				m.Files = make(map[string]string)
			}
			var mapkey string
			var mapvalue string
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowBroker
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					wire |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				fieldNum := int32(wire >> 3)
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowBroker
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return ErrInvalidLengthBroker
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey < 0 {
						return ErrInvalidLengthBroker
					}
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var stringLenmapvalue uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowBroker
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapvalue |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapvalue := int(stringLenmapvalue)
					if intStringLenmapvalue < 0 {
						return ErrInvalidLengthBroker
					}
					postStringIndexmapvalue := iNdEx + intStringLenmapvalue
					if postStringIndexmapvalue < 0 {
						return ErrInvalidLengthBroker
					}
					if postStringIndexmapvalue > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = string(dAtA[iNdEx:postStringIndexmapvalue])
					iNdEx = postStringIndexmapvalue
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipBroker(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if skippy < 0 {
						return ErrInvalidLengthBroker
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.Files[mapkey] = mapvalue
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field EnvironmentURL", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthBroker
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthBroker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.EnvironmentURL = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Trace", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthBroker
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthBroker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Trace = append(m.Trace[:0], dAtA[iNdEx:postIndex]...)
			if m.Trace == nil {
				m.Trace = []byte{}
			}
			iNdEx = postIndex
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TraceID", wireType)
			}
			m.TraceID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TraceID |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sync", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Sync = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipBroker(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthBroker
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthBroker
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipBroker(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowBroker
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowBroker
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthBroker
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthBroker
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowBroker
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipBroker(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthBroker
				}
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthBroker = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowBroker   = fmt.Errorf("proto: integer overflow")
)
