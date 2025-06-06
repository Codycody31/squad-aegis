// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v6.30.1
// source: logwatcher.proto

package logwatcher

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	LogWatcher_StreamLogs_FullMethodName   = "/logwatcher.LogWatcher/StreamLogs"
	LogWatcher_StreamEvents_FullMethodName = "/logwatcher.LogWatcher/StreamEvents"
)

// LogWatcherClient is the client API for LogWatcher service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type LogWatcherClient interface {
	StreamLogs(ctx context.Context, in *AuthRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LogEntry], error)
	StreamEvents(ctx context.Context, in *AuthRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[EventEntry], error)
}

type logWatcherClient struct {
	cc grpc.ClientConnInterface
}

func NewLogWatcherClient(cc grpc.ClientConnInterface) LogWatcherClient {
	return &logWatcherClient{cc}
}

func (c *logWatcherClient) StreamLogs(ctx context.Context, in *AuthRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LogEntry], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LogWatcher_ServiceDesc.Streams[0], LogWatcher_StreamLogs_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[AuthRequest, LogEntry]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LogWatcher_StreamLogsClient = grpc.ServerStreamingClient[LogEntry]

func (c *logWatcherClient) StreamEvents(ctx context.Context, in *AuthRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[EventEntry], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &LogWatcher_ServiceDesc.Streams[1], LogWatcher_StreamEvents_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[AuthRequest, EventEntry]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LogWatcher_StreamEventsClient = grpc.ServerStreamingClient[EventEntry]

// LogWatcherServer is the server API for LogWatcher service.
// All implementations must embed UnimplementedLogWatcherServer
// for forward compatibility.
type LogWatcherServer interface {
	StreamLogs(*AuthRequest, grpc.ServerStreamingServer[LogEntry]) error
	StreamEvents(*AuthRequest, grpc.ServerStreamingServer[EventEntry]) error
	mustEmbedUnimplementedLogWatcherServer()
}

// UnimplementedLogWatcherServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedLogWatcherServer struct{}

func (UnimplementedLogWatcherServer) StreamLogs(*AuthRequest, grpc.ServerStreamingServer[LogEntry]) error {
	return status.Errorf(codes.Unimplemented, "method StreamLogs not implemented")
}
func (UnimplementedLogWatcherServer) StreamEvents(*AuthRequest, grpc.ServerStreamingServer[EventEntry]) error {
	return status.Errorf(codes.Unimplemented, "method StreamEvents not implemented")
}
func (UnimplementedLogWatcherServer) mustEmbedUnimplementedLogWatcherServer() {}
func (UnimplementedLogWatcherServer) testEmbeddedByValue()                    {}

// UnsafeLogWatcherServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LogWatcherServer will
// result in compilation errors.
type UnsafeLogWatcherServer interface {
	mustEmbedUnimplementedLogWatcherServer()
}

func RegisterLogWatcherServer(s grpc.ServiceRegistrar, srv LogWatcherServer) {
	// If the following call pancis, it indicates UnimplementedLogWatcherServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&LogWatcher_ServiceDesc, srv)
}

func _LogWatcher_StreamLogs_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(AuthRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LogWatcherServer).StreamLogs(m, &grpc.GenericServerStream[AuthRequest, LogEntry]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LogWatcher_StreamLogsServer = grpc.ServerStreamingServer[LogEntry]

func _LogWatcher_StreamEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(AuthRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LogWatcherServer).StreamEvents(m, &grpc.GenericServerStream[AuthRequest, EventEntry]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type LogWatcher_StreamEventsServer = grpc.ServerStreamingServer[EventEntry]

// LogWatcher_ServiceDesc is the grpc.ServiceDesc for LogWatcher service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LogWatcher_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "logwatcher.LogWatcher",
	HandlerType: (*LogWatcherServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamLogs",
			Handler:       _LogWatcher_StreamLogs_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "StreamEvents",
			Handler:       _LogWatcher_StreamEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "logwatcher.proto",
}
