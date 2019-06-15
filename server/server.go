package server

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/jiajunhuang/natproxy/errors"
	"github.com/jiajunhuang/natproxy/pb"
	"github.com/jiajunhuang/natproxy/tools"
	reuse "github.com/libp2p/go-reuseport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var (
	certFilePath = flag.String("certPath", "/root/.acme.sh/*.laizuoceshi.com/*.laizuoceshi.com.cer", "cert file path")
	keyFilePath  = flag.String("keyPath", "/root/.acme.sh/*.laizuoceshi.com/*.laizuoceshi.com.key", "key file path")
)

// Start gRPC server
func Start(addr, wanIP string, bufSize int) {
	listener, err := reuse.Listen("tcp", addr)
	if err != nil {
		log.Printf("failed to listen at addr %s", addr)
	}

	// register service
	svc := newService(wanIP, bufSize)
	creds, err := credentials.NewServerTLSFromFile(*certFilePath, *keyFilePath)
	if err != nil {
		log.Fatalf("failed to create credentials: %v", err)
	}
	server := grpc.NewServer(grpc.Creds(creds))

	pb.RegisterServerServiceServer(server, svc)
	log.Printf("server start to listen at %s, WAN ip is %s, bufSize is %d", addr, wanIP, bufSize)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %s", err)
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
	token := getToken(ctx)

	// 获取客户端信息
	client, ok := peer.FromContext(ctx)
	log.Printf("client(%s) connected, ok: %t", client, ok)
	defer log.Printf("client(%s) disconnected", client)

	// 启动公网端口监听 && 下发消息给客户端告知公网地址
	wanListener, wanListenerAddr, err := s.getWANListen(ctx)
	if err != nil {
		log.Printf("failed to create listener for WAN: %s", err)
		return err
	}
	defer wanListener.Close()
	log.Printf("client(%s, token: %s)'s WAN listener listen at %s", client, token, wanListenerAddr)
	go manager.receiveConnFromWAN(client, wanListener)
	manager.msgCh <- &pb.MsgResponse{Type: pb.MsgType_WANAddr, Data: []byte(wanListenerAddr)}

	// 启动客户端监听
	// ref: https://en.wikipedia.org/wiki/Ephemeral_port 一般Linux的port范围是32768 ~ 61000
	clientListener, clientListenerAddr, err := s.createListenerByPort("0")
	if err != nil {
		log.Printf("failed to create listener for client: %s", err)
		return err
	}
	defer clientListener.Close()
	log.Printf("client(%s, token: %s) listener listen at %s", client, token, clientListenerAddr)
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
				log.Printf("message(to client) channel closed")
				return errors.ErrMsgChanClosed
			}

			if err := stream.Send(msg); err != nil {
				log.Printf("failed to send message(%s): %s", msg, err)
			}
			log.Printf("successfully send message(%s) to client", msg)
		case msg, ok := <-manager.clientMsgCh:
			if !ok {
				return errors.ErrMsgChanClosed
			}
			switch msg.Type {
			case pb.MsgType_DisConnect:
				log.Printf("client(%s, token: %s) ask me to disconnect", client, token)
				return nil
			case pb.MsgType_Report:
				var clientInfo pb.ClientInfo
				if err = proto.Unmarshal(msg.Data, &clientInfo); err != nil {
					log.Printf("failed to unmarshal client info %s: %s", msg.Data, err)
				}
				log.Printf("client(%s, token: %s) report info %+v", client, token, clientInfo)
			default:
				log.Printf("client send bad message %s", msg)
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
		log.Printf("trying to listen port %d", port)
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			log.Printf("port(%d) can't be listened: %s", port, err)
			retry++
			continue
		}
		l.Close()
		log.Printf("port(%d) is ok to listen, try to check if the port is already taken by others", port)

		// 检查一下是否被其他用户分配过
		addr = fmt.Sprintf("%s:%d", s.wanIP, port)
		taken, err := tools.CheckIfAddrAlreadyTaken(addr)
		if err != nil {
			log.Printf("failed to check if addr(%s) already been taken by others: %s", addr, err)
			return "", err
		}

		if taken {
			log.Printf("addr(%s) had been taken", addr)
			retry++
			continue
		}

		if err = tools.RegisterAddr(token, addr); err != nil {
			log.Printf("failed to register addr %s: %s", addr, err)
			return "", err
		}

		return addr, nil
	}
}

// 根据给定的地址创建一个监听器
func (s *service) createListenerByPort(port string) (net.Listener, string, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.Printf("failed to listen at 0.0.0.0:%s: %s", port, err)
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
		log.Printf("bad metadata %s", md)
		return ""
	}
	token := md.Get("natproxy-token")
	if len(token) != 1 {
		log.Printf("bad token(%s) in metadata", token)
		return ""
	}

	return token[0]
}
