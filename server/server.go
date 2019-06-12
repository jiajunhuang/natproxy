package server

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/jiajunhuang/natproxy/errors"
	"github.com/jiajunhuang/natproxy/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	logger, _ = zap.NewProduction()
	wanip     = ""
)

func Start(addr, wanIP string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatal("failed to listen", zap.String("addr", addr))
	}

	// register service
	svc := newService(wanIP)
	server := grpc.NewServer()

	pb.RegisterServerServiceServer(server, svc)
	logger.Info("server start to listen", zap.String("addr", addr))
	if err := server.Serve(listener); err != nil {
		logger.Fatal("failed to serve", zap.Error(err))
	}
}

type service struct {
	wanIP string
}

func newService(wanIP string) *service {
	return &service{
		wanIP: wanIP,
	}
}

func (s *service) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return nil, nil
}

func (s *service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return nil, nil
}

func (s *service) Msg(stream pb.ServerService_MsgServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		logger.Error("bad metadata", zap.Any("metadata", md))
		return errors.ErrBadMetadata
	}
	token := md.Get("natrp-token")
	if len(token) != 1 {
		logger.Error("bad token in metadata", zap.Any("token", token))
		return errors.ErrBadMetadata
	}

	listenAddr := getListenAddrByToken(token[0])

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("failed to listen", zap.Error(err))
		return err
	}
	defer listener.Close()
	addrList := strings.Split(listener.Addr().String(), ":")
	addr := fmt.Sprintf("%s:%s", s.wanIP, addrList[len(addrList)-1])
	logger.Info("server listen at", zap.String("addr", addr))

	conn, err := listener.Accept()
	if err != nil {
		logger.Error("failed to accept", zap.Error(err))
		return err
	}
	defer conn.Close()

	go func() {
		defer conn.Close()

		for {
			req, err := stream.Recv()
			if err != nil {
				logger.Error("failed to read", zap.Error(err))
				return
			}

			if _, err := conn.Write(req.Data); err != nil {
				logger.Error("failed to write", zap.Error(err))
				return
			}
		}
	}()

	data := make([]byte, 1024)
	for {
		n, err := conn.Read(data)
		if err != nil {
			logger.Error("failed to read", zap.Error(err))
			return err
		}

		if err := stream.Send(&pb.MsgResponse{Data: data[:n]}); err != nil {
			logger.Error("failed to write", zap.Error(err))
			return err
		}
	}
}

func getListenAddrByToken(token string) string {
	return "0.0.0.0:10033"
}
