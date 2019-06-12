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
	"google.golang.org/grpc/peer"
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
	wanIP        string
	wanConnCh    chan net.Conn // connections from WAN
	clientConnCh chan net.Conn // connections from client
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
	defer close(s.wanConnCh)

	ctx := stream.Context()

	remote, ok := peer.FromContext(ctx)
	logger.Info("client connected", zap.Any("remote", remote), zap.Bool("ok", ok))

	// get listener for current user
	wanListener, wanListenerAddr, err := s.getWANListen(ctx)
	if err != nil {
		logger.Error("failed to create listener for WAN ", zap.Error(err))
		return err
	}
	defer wanListener.Close()
	// TODO 下发消息给客户端告知公网地址
	_ = wanListenerAddr
	go func() {
		for {
			conn, err := wanListener.Accept()
			if err != nil {
				logger.Error("failed to accept new connection for WAN", zap.Any("remote", remote), zap.Error(err))
				break
			}
			s.wanConnCh <- conn
		}
	}()

	// create a listener for client
	clientListener, clientListenerAddr, err := s.createListener("0.0.0.0:0")
	if err != nil {
		logger.Error("failed to create listener for client", zap.Error(err))
		return err
	}
	defer clientListener.Close()
	// TODO 下发消息给客户端告知客户端应该连接的公网地址
	_ = clientListenerAddr
	go func() {
		for {
			conn, err := clientListener.Accept()
			if err != nil {
				logger.Error("failed to accept new connection for client", zap.Any("remote", remote), zap.Error(err))
				break
			}
			s.clientConnCh <- conn
		}
		logger.Info("listener for client closing...")
	}()

	for {
		conn, ok := <-s.wanConnCh
		if !ok {
			logger.Error("connection channel closed")
			return errors.ErrConnectionChClosed
		}

		go s.handleWANRequest(conn)
	}
}

func (s *service) getWANListen(ctx context.Context) (net.Listener, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Error("bad metadata", zap.Any("metadata", md))
		return nil, "", errors.ErrBadMetadata
	}
	token := md.Get("natrp-token")
	if len(token) != 1 {
		logger.Error("bad token in metadata", zap.Any("token", token))
		return nil, "", errors.ErrBadMetadata
	}

	listenAddr := getListenAddrByToken(token[0])

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("failed to listen", zap.Error(err))
		return nil, "", err
	}
	addrList := strings.Split(listener.Addr().String(), ":")
	addr := fmt.Sprintf("%s:%s", s.wanIP, addrList[len(addrList)-1])
	logger.Info("server listen at", zap.String("addr", addr))

	return listener, addr, nil
}

func getListenAddrByToken(token string) string {
	return "0.0.0.0:10033"
}

func (s *service) createListener(addr string) (net.Listener, string, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", zap.Error(err))
		return nil, "", err
	}
	addrList := strings.Split(listener.Addr().String(), ":")
	listenerAddr := fmt.Sprintf("%s:%s", s.wanIP, addrList[len(addrList)-1])
	logger.Info("server listen at", zap.String("addr", addr))

	return listener, listenerAddr, nil
}

func (s *service) handleWANRequest(wanConn net.Conn) {
	// 下发消息给客户端要求建立新的connection

	// 等待新的connection

	// 把两个connection串起来
}
