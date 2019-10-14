// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: pkg/lb/proto/runner.proto

package proto

import (
	context "context"
	fmt "fmt"
	proto1 "github.com/Syncano/codebox/pkg/script/proto"
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
	// Types that are valid to be assigned to Value:
	//	*RunRequest_Meta
	//	*RunRequest_Request
	Value                isRunRequest_Value `protobuf_oneof:"value"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *RunRequest) Reset()         { *m = RunRequest{} }
func (m *RunRequest) String() string { return proto.CompactTextString(m) }
func (*RunRequest) ProtoMessage()    {}
func (*RunRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_dc16abcd905d9171, []int{0}
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

type isRunRequest_Value interface {
	isRunRequest_Value()
	MarshalTo([]byte) (int, error)
	Size() int
}

type RunRequest_Meta struct {
	Meta *RunRequest_MetaMessage `protobuf:"bytes,1,opt,name=meta,proto3,oneof"`
}
type RunRequest_Request struct {
	Request *proto1.RunRequest `protobuf:"bytes,2,opt,name=request,proto3,oneof"`
}

func (*RunRequest_Meta) isRunRequest_Value()    {}
func (*RunRequest_Request) isRunRequest_Value() {}

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

func (m *RunRequest) GetRequest() *proto1.RunRequest {
	if x, ok := m.GetValue().(*RunRequest_Request); ok {
		return x.Request
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*RunRequest) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _RunRequest_OneofMarshaler, _RunRequest_OneofUnmarshaler, _RunRequest_OneofSizer, []interface{}{
		(*RunRequest_Meta)(nil),
		(*RunRequest_Request)(nil),
	}
}

func _RunRequest_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*RunRequest)
	// value
	switch x := m.Value.(type) {
	case *RunRequest_Meta:
		_ = b.EncodeVarint(1<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Meta); err != nil {
			return err
		}
	case *RunRequest_Request:
		_ = b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Request); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("RunRequest.Value has unexpected type %T", x)
	}
	return nil
}

func _RunRequest_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*RunRequest)
	switch tag {
	case 1: // value.meta
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(RunRequest_MetaMessage)
		err := b.DecodeMessage(msg)
		m.Value = &RunRequest_Meta{msg}
		return true, err
	case 2: // value.request
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(proto1.RunRequest)
		err := b.DecodeMessage(msg)
		m.Value = &RunRequest_Request{msg}
		return true, err
	default:
		return false, nil
	}
}

func _RunRequest_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*RunRequest)
	// value
	switch x := m.Value.(type) {
	case *RunRequest_Meta:
		s := proto.Size(x.Meta)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *RunRequest_Request:
		s := proto.Size(x.Request)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

// Meta message specifies fields to describe what is being run.
type RunRequest_MetaMessage struct {
	RequestID            string   `protobuf:"bytes,3,opt,name=requestID,proto3" json:"requestID,omitempty"`
	ConcurrencyKey       string   `protobuf:"bytes,1,opt,name=concurrencyKey,proto3" json:"concurrencyKey,omitempty"`
	ConcurrencyLimit     int32    `protobuf:"varint,2,opt,name=concurrencyLimit,proto3" json:"concurrencyLimit,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RunRequest_MetaMessage) Reset()         { *m = RunRequest_MetaMessage{} }
func (m *RunRequest_MetaMessage) String() string { return proto.CompactTextString(m) }
func (*RunRequest_MetaMessage) ProtoMessage()    {}
func (*RunRequest_MetaMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_dc16abcd905d9171, []int{0, 0}
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

func (m *RunRequest_MetaMessage) GetRequestID() string {
	if m != nil {
		return m.RequestID
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetConcurrencyKey() string {
	if m != nil {
		return m.ConcurrencyKey
	}
	return ""
}

func (m *RunRequest_MetaMessage) GetConcurrencyLimit() int32 {
	if m != nil {
		return m.ConcurrencyLimit
	}
	return 0
}

func init() {
	proto.RegisterType((*RunRequest)(nil), "lb.RunRequest")
	proto.RegisterType((*RunRequest_MetaMessage)(nil), "lb.RunRequest.MetaMessage")
}

func init() { proto.RegisterFile("pkg/lb/proto/runner.proto", fileDescriptor_dc16abcd905d9171) }

var fileDescriptor_dc16abcd905d9171 = []byte{
	// 304 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x91, 0xbf, 0x4e, 0x42, 0x31,
	0x14, 0xc6, 0x29, 0x88, 0x84, 0x83, 0x21, 0xa6, 0x2e, 0x78, 0xa3, 0x37, 0xc4, 0x41, 0x89, 0x43,
	0x2f, 0xc1, 0xc9, 0xc5, 0x81, 0x38, 0x60, 0x94, 0xa5, 0x6c, 0x6e, 0xb7, 0xf5, 0x04, 0x6f, 0xbc,
	0xb4, 0xd7, 0xfe, 0x31, 0x32, 0xf9, 0x1a, 0x3e, 0x92, 0xa3, 0x8f, 0x60, 0xf0, 0x25, 0x1c, 0x8d,
	0xbd, 0x18, 0x40, 0xa7, 0xe6, 0x7c, 0xdf, 0xef, 0xb4, 0xdf, 0xe9, 0x81, 0xfd, 0xe2, 0x61, 0x9a,
	0xe4, 0x22, 0x29, 0x8c, 0x76, 0x3a, 0x31, 0x5e, 0x29, 0x34, 0x2c, 0x14, 0xb4, 0x9a, 0x8b, 0xe8,
	0xf0, 0xc7, 0xb6, 0xd2, 0x64, 0x85, 0x5b, 0x22, 0x65, 0x51, 0x22, 0x47, 0x5f, 0x04, 0x80, 0x7b,
	0xc5, 0xf1, 0xd1, 0xa3, 0x75, 0xb4, 0x0f, 0x5b, 0x33, 0x74, 0x69, 0x87, 0x74, 0x49, 0xaf, 0x35,
	0x88, 0x58, 0x2e, 0xd8, 0xca, 0x65, 0x63, 0x74, 0xe9, 0x18, 0xad, 0x4d, 0xa7, 0x38, 0xaa, 0xf0,
	0x40, 0x52, 0x06, 0x0d, 0x53, 0xda, 0x9d, 0x6a, 0x68, 0xa2, 0x6c, 0xf9, 0xc0, 0xaa, 0x71, 0x54,
	0xe1, 0xbf, 0x50, 0xf4, 0x02, 0xad, 0xb5, 0x6b, 0xe8, 0x01, 0x34, 0x97, 0xce, 0xd5, 0x65, 0xa7,
	0xd6, 0x25, 0xbd, 0x26, 0x5f, 0x09, 0xf4, 0x18, 0xda, 0x52, 0x2b, 0xe9, 0x8d, 0x41, 0x25, 0xe7,
	0xd7, 0x38, 0x0f, 0xc1, 0x9a, 0xfc, 0x8f, 0x4a, 0x4f, 0x61, 0x77, 0x4d, 0xb9, 0xc9, 0x66, 0x59,
	0x99, 0xa6, 0xce, 0xff, 0xe9, 0xc3, 0x06, 0xd4, 0x9f, 0xd2, 0xdc, 0xe3, 0xe0, 0x02, 0x76, 0x26,
	0x21, 0x29, 0x0f, 0x7f, 0x46, 0x19, 0xd4, 0xb8, 0x57, 0xb4, 0xbd, 0x39, 0x74, 0xb4, 0xb7, 0x31,
	0x8f, 0x2d, 0xb4, 0xb2, 0xd8, 0x23, 0x7d, 0x32, 0x3c, 0x7f, 0x5b, 0xc4, 0xe4, 0x7d, 0x11, 0x93,
	0x8f, 0x45, 0x4c, 0x5e, 0x3f, 0xe3, 0xca, 0xed, 0xc9, 0x34, 0x73, 0xf7, 0x5e, 0x30, 0xa9, 0x67,
	0xc9, 0x64, 0xae, 0x64, 0xaa, 0x74, 0x22, 0xf5, 0x1d, 0x0a, 0xfd, 0x9c, 0xac, 0x6f, 0x49, 0x6c,
	0x87, 0xe3, 0xec, 0x3b, 0x00, 0x00, 0xff, 0xff, 0xe1, 0x4a, 0x4c, 0xfb, 0xbc, 0x01, 0x00, 0x00,
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
	stream, err := c.cc.NewStream(ctx, &_ScriptRunner_serviceDesc.Streams[0], "/lb.ScriptRunner/Run", opts...)
	if err != nil {
		return nil, err
	}
	x := &scriptRunnerRunClient{stream}
	return x, nil
}

type ScriptRunner_RunClient interface {
	Send(*RunRequest) error
	Recv() (*proto1.RunResponse, error)
	grpc.ClientStream
}

type scriptRunnerRunClient struct {
	grpc.ClientStream
}

func (x *scriptRunnerRunClient) Send(m *RunRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *scriptRunnerRunClient) Recv() (*proto1.RunResponse, error) {
	m := new(proto1.RunResponse)
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
	Send(*proto1.RunResponse) error
	Recv() (*RunRequest, error)
	grpc.ServerStream
}

type scriptRunnerRunServer struct {
	grpc.ServerStream
}

func (x *scriptRunnerRunServer) Send(m *proto1.RunResponse) error {
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
	ServiceName: "lb.ScriptRunner",
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
	Metadata: "pkg/lb/proto/runner.proto",
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
	if m.Value != nil {
		nn1, err := m.Value.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += nn1
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func (m *RunRequest_Meta) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.Meta != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintRunner(dAtA, i, uint64(m.Meta.Size()))
		n2, err := m.Meta.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	return i, nil
}
func (m *RunRequest_Request) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.Request != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintRunner(dAtA, i, uint64(m.Request.Size()))
		n3, err := m.Request.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
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
	if len(m.ConcurrencyKey) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintRunner(dAtA, i, uint64(len(m.ConcurrencyKey)))
		i += copy(dAtA[i:], m.ConcurrencyKey)
	}
	if m.ConcurrencyLimit != 0 {
		dAtA[i] = 0x10
		i++
		i = encodeVarintRunner(dAtA, i, uint64(m.ConcurrencyLimit))
	}
	if len(m.RequestID) > 0 {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintRunner(dAtA, i, uint64(len(m.RequestID)))
		i += copy(dAtA[i:], m.RequestID)
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeVarintRunner(dAtA []byte, offset int, v uint64) int {
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
	if m.Value != nil {
		n += m.Value.Size()
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *RunRequest_Meta) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Meta != nil {
		l = m.Meta.Size()
		n += 1 + l + sovRunner(uint64(l))
	}
	return n
}
func (m *RunRequest_Request) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Request != nil {
		l = m.Request.Size()
		n += 1 + l + sovRunner(uint64(l))
	}
	return n
}
func (m *RunRequest_MetaMessage) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ConcurrencyKey)
	if l > 0 {
		n += 1 + l + sovRunner(uint64(l))
	}
	if m.ConcurrencyLimit != 0 {
		n += 1 + sovRunner(uint64(m.ConcurrencyLimit))
	}
	l = len(m.RequestID)
	if l > 0 {
		n += 1 + l + sovRunner(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovRunner(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozRunner(x uint64) (n int) {
	return sovRunner(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *RunRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowRunner
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
					return ErrIntOverflowRunner
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
				return ErrInvalidLengthRunner
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRunner
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &RunRequest_MetaMessage{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Value = &RunRequest_Meta{v}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Request", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRunner
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
				return ErrInvalidLengthRunner
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRunner
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &proto1.RunRequest{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Value = &RunRequest_Request{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipRunner(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthRunner
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthRunner
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
				return ErrIntOverflowRunner
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
				return fmt.Errorf("proto: wrong wireType = %d for field ConcurrencyKey", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRunner
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
				return ErrInvalidLengthRunner
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthRunner
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ConcurrencyKey = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ConcurrencyLimit", wireType)
			}
			m.ConcurrencyLimit = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRunner
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ConcurrencyLimit |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RequestID", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRunner
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
				return ErrInvalidLengthRunner
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthRunner
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.RequestID = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipRunner(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthRunner
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthRunner
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
func skipRunner(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowRunner
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
					return 0, ErrIntOverflowRunner
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
					return 0, ErrIntOverflowRunner
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
				return 0, ErrInvalidLengthRunner
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthRunner
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowRunner
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
				next, err := skipRunner(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthRunner
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
	ErrInvalidLengthRunner = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowRunner   = fmt.Errorf("proto: integer overflow")
)
