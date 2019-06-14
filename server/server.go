package server

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/jiajunhuang/natproxy/errors"
	"github.com/jiajunhuang/natproxy/pb"
	"github.com/jiajunhuang/natproxy/tools"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var (
	logger, _ = zap.NewProduction()
)

// Start gRPC server
func Start(addr, wanIP string, bufSize int) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatal("failed to listen", zap.String("addr", addr))
	}

	// register service
	svc := newService(wanIP, bufSize)
	server := grpc.NewServer()

	pb.RegisterServerServiceServer(server, svc)
	logger.Info("server start to listen", zap.String("addr", addr), zap.String("wanip", wanIP), zap.Int("bufSize", bufSize))
	if err := server.Serve(listener); err != nil {
		logger.Fatal("failed to serve", zap.Error(err))
	}
}

type service struct {
	wanIP   string
	bufSize int
}

func newService(wanIP string, bufSize int) *service {
	return &service{
		wanIP:   wanIP,
		bufSize: bufSize,
	}
}

func (s *service) Msg(stream pb.ServerService_MsgServer) error {
	manager := newManager(s, s.bufSize)
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
	logger.Info("WAN listener listen at", zap.String("wan addr", wanListenerAddr), zap.Any("client", client), zap.String("token", getToken(ctx)))
	go manager.receiveConnFromWAN(client, wanListener)
	manager.msgCh <- &pb.MsgResponse{Type: pb.MsgType_WANAddr, Data: []byte(wanListenerAddr)}

	// 启动客户端监听
	// ref: https://en.wikipedia.org/wiki/Ephemeral_port 一般Linux的port范围是32768 ~ 61000
	clientListener, clientListenerAddr, err := s.createListenerByPort("0")
	if err != nil {
		logger.Error("failed to create listener for client", zap.Error(err))
		return err
	}
	defer clientListener.Close()
	logger.Info("client listener listen at", zap.String("client addr", wanListenerAddr), zap.Any("client", client), zap.String("token", getToken(ctx)))
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
			case pb.MsgType_Report:
				var clientInfo pb.ClientInfo
				if err = proto.Unmarshal(msg.Data, &clientInfo); err != nil {
					logger.Error("failed to unmarshal client info", zap.ByteString("data", msg.Data), zap.Error(err))
				}
				logger.Info("client report info", zap.Any("info", clientInfo), zap.Any("client", client))
			default:
				logger.Error("client send bad message", zap.Any("msg", msg))
			}
		}
	}
}

// 获得公网监听
func (s *service) getWANListen(ctx context.Context) (net.Listener, string, error) {
	token := getToken(ctx)

	listenAddr, err := s.getListenAddrByToken(token)
	if err != nil {
		return nil, listenAddr, err
	}
	addrList := strings.Split(listenAddr, ":")
	port := addrList[len(addrList)-1]

	return s.createListenerByPort(port)
}

// 根据token查询
func (s *service) getListenAddrByToken(token string) (string, error) {
	addr, err := tools.GetAddrByToken(token)
	if err != nil {
		return "", err
	}

	// 如果已经分配过公网地址
	if addr != "" {
		addrList := strings.Split(addr, ":")
		// 如果上次分配的地址是本机，那么直接返回，否则，就应该重新分配
		if addrList[0] == s.wanIP {
			listenerAddr := fmt.Sprintf("0.0.0.0:%s", addrList[len(addrList)-1])
			return listenerAddr, nil
		}
	}

	// 没有分配过公网监听地址，那就在 15000 ~ 32767 之间分配一个
	retry := 0
	for {
		if retry > 20 {
			return "", errors.ErrFailedToAllocatePort
		}

		port := s.getRandomPort()
		logger.Info("trying to listen port", zap.Int("port", port))
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			logger.Error("port can't be listened", zap.Int("port", port), zap.Error(err))
			retry++
			continue
		}
		l.Close()
		logger.Info("port is ok to listen, try to check if the port is already taken by others", zap.Int("port", port))

		// 检查一下是否被其他用户分配过
		addr = fmt.Sprintf("%s:%d", s.wanIP, port)
		taken, err := tools.CheckIfAddrAlreadyTaken(addr)
		if err != nil {
			logger.Error("failed to check if addr already been taken by others", zap.String("addr", addr), zap.Error(err))
			return "", err
		}

		if taken {
			logger.Error("addr had been taken", zap.String("addr", addr))
			retry++
			continue
		}

		if err = tools.RegisterAddr(token, addr); err != nil {
			logger.Error("failed to register addr", zap.String("addr", addr), zap.Error(err))
			return "", err
		}

		return addr, nil
	}
}

// 根据给定的地址创建一个监听器
func (s *service) createListenerByPort(port string) (net.Listener, string, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		logger.Error("failed to listen", zap.Error(err))
		return nil, "", err
	}
	addrList := strings.Split(listener.Addr().String(), ":")
	port = addrList[len(addrList)-1]

	return listener, fmt.Sprintf("%s:%s", s.wanIP, port), nil
}

// 没有分配过公网监听地址，那就在 15000 ~ 32767 之间分配一个
func (s *service) getRandomPort() int {
	max := 32767
	min := 15000

	return rand.Intn(max-min) + min
}

func getToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logger.Error("bad metadata", zap.Any("metadata", md))
		return ""
	}
	token := md.Get("natrp-token")
	if len(token) != 1 {
		logger.Error("bad token in metadata", zap.Any("token", token))
		return ""
	}

	return token[0]
}
