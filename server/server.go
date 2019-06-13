package server

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/jiajunhuang/natproxy/dial"
	"github.com/jiajunhuang/natproxy/errors"
	"github.com/jiajunhuang/natproxy/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var (
	logger, _ = zap.NewProduction()
)

// Start gRPC server
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

type manager struct {
	service      *service
	wanConnCh    chan net.Conn        // connections from WAN
	clientConnCh chan net.Conn        // connections from client
	msgCh        chan *pb.MsgResponse // messages send to client
	clientMsgCh  chan *pb.MsgRequest  // messages from client
}

func newService(wanIP string) *service {
	return &service{
		wanIP: wanIP,
	}
}

func newManager(svc *service) *manager {
	return &manager{
		service:      svc,
		wanConnCh:    make(chan net.Conn, 16),
		clientConnCh: make(chan net.Conn, 16),
		msgCh:        make(chan *pb.MsgResponse, 16),
		clientMsgCh:  make(chan *pb.MsgRequest, 16),
	}
}

func (s *service) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return nil, nil
}

func (s *service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return nil, nil
}

func (s *service) Msg(stream pb.ServerService_MsgServer) error {
	manager := newManager(s)
	defer close(manager.msgCh)

	ctx := stream.Context()

	// 获取客户端信息
	client, ok := peer.FromContext(ctx)
	logger.Info("client connected", zap.Any("client", client), zap.Bool("ok", ok))
	defer logger.Info("client disconnected", zap.Any("client", client))

	// 启动公网端口监听 && 下发消息给客户端告知公网地址
	wanListener, wanListenerAddr, err := s.getWANListen(ctx)
	if err != nil {
		logger.Error("failed to create listener for WAN ", zap.Error(err))
		return err
	}
	defer wanListener.Close()
	go manager.receiveConnFromWAN(client, wanListener)
	manager.msgCh <- &pb.MsgResponse{Type: pb.MsgType_WANAddr, Data: []byte(wanListenerAddr)}

	// 启动客户端监听
	clientListener, clientListenerAddr, err := s.createListener("0.0.0.0:0")
	if err != nil {
		logger.Error("failed to create listener for client", zap.Error(err))
		return err
	}
	defer clientListener.Close()
	go manager.receiveConnFromClient(client, clientListener)

	// 处理来自公网请求
	go manager.handleConnFromWAN(clientListenerAddr)

	// 接收来自客户端的gRPC请求
	go manager.receiveMsgFromClient(stream)

	// 启动客户端下发消息器
	for {
		select {
		case msg, ok := <-manager.msgCh:
			if !ok {
				logger.Error("message(to client) channel closed")
				return errors.ErrMsgChanClosed
			}

			if err := stream.Send(msg); err != nil {
				logger.Error("failed to send message", zap.Any("msg", msg), zap.Error(err))
			}
			logger.Info("successfully send message to client", zap.Any("msg", msg))
		case msg, ok := <-manager.clientMsgCh:
			if !ok {
				return errors.ErrMsgChanClosed
			}
			switch msg.Type {
			case pb.MsgType_DisConnect:
				logger.Warn("client is closing, so I'm quit...")
				return nil
			default:
				logger.Error("client send bad message", zap.Any("msg", msg))
			}
		}
	}
}

// 获得公网监听
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

	return s.createListener(listenAddr)
}

// 这里应该改成根据数据库查
func getListenAddrByToken(token string) string {
	return "0.0.0.0:10033"
}

// 根据给定的地址创建一个监听器
func (s *service) createListener(addr string) (net.Listener, string, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", zap.Error(err))
		return nil, "", err
	}
	addrList := strings.Split(listener.Addr().String(), ":")
	listenerAddr := fmt.Sprintf("%s:%s", s.wanIP, addrList[len(addrList)-1])
	logger.Info("server listen at", zap.String("addr", listenerAddr))

	return listener, listenerAddr, nil
}

// 客户端消息接收器
func (manager *manager) receiveMsgFromClient(stream pb.ServerService_MsgServer) {
	defer close(manager.clientMsgCh)

	for {
		req, err := stream.Recv()
		if err != nil {
			return
		}

		manager.clientMsgCh <- req
	}
}

// 公网请求处理器
func (manager *manager) handleConnFromWAN(clientListenerAddr string) {
	for {
		wanConn, ok := <-manager.wanConnCh
		if !ok {
			return
		}

		go func() {
			// 下发消息给客户端要求建立新的connection
			manager.msgCh <- &pb.MsgResponse{Type: pb.MsgType_Connect, Data: []byte(clientListenerAddr)}

			// 等待新的connection
			clientConn, ok := <-manager.clientConnCh
			if !ok {
				logger.Error("failed to receive connection from client connection channel")
				return
			}
			wanConnAddr, clientConnAddr := wanConn.LocalAddr(), clientConn.RemoteAddr()
			defer logger.Info("connection between WAN & client disconnected", zap.Any("wanConn", wanConnAddr), zap.Any("clientConn", clientConnAddr))

			// 把两个connection串起来
			dial.Join(wanConn, clientConn)
		}()
	}
}

// 接收来自客户端的请求并且传递给channel
func (manager *manager) receiveConnFromClient(client *peer.Peer, clientListener net.Listener) {
	defer close(manager.clientConnCh)

	logger.Info("start to wait new connections from client...")
	for {
		conn, err := clientListener.Accept()
		if err != nil {
			return
		}
		manager.clientConnCh <- conn
	}
}

// 接收来自公网的请求并且传递给channel
func (manager *manager) receiveConnFromWAN(client *peer.Peer, wanListener net.Listener) {
	defer close(manager.wanConnCh)

	logger.Info("start to wait new connections from WAN...")
	for {
		conn, err := wanListener.Accept()
		if err != nil {
			return
		}
		manager.wanConnCh <- conn
	}
}
