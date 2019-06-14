package client

import (
	"context"
	"flag"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/jiajunhuang/natproxy/dial"
	"github.com/jiajunhuang/natproxy/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

const (
	version = "0.0.2"
	arch    = runtime.GOARCH
	os      = runtime.GOOS
)

var (
	logger, _ = zap.NewProduction()

	localAddr  = flag.String("local", "127.0.0.1:80", "-local=<你本地需要转发的地址>")
	serverAddr = flag.String("server", "127.0.0.1:10020", "-server=<你的服务器地址>")
	token      = flag.String("token", "balalaxiaomoxian", "-token=<你的token>")
	useTLS     = flag.Bool("tls", true, "-tls=true 默认使用TLS加密")
)

func connectServer(addr string) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		logger.Error("failed to dial with server", zap.String("addr", addr), zap.Error(err))
		return
	}

	localConn, err := net.Dial("tcp", *localAddr)
	if err != nil {
		logger.Error("failed to dial with local address", zap.String("addr", *localAddr), zap.Error(err))
		return
	}

	dial.Join(conn, localConn)
}

func waitMsgFromServer(addr string) error {
	md := metadata.Pairs("natrp-token", *token)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client, conn, err := dial.WithServer(ctx, *serverAddr, *useTLS)
	if err != nil {
		logger.Error("failed to connect to server server", zap.Error(err))
		return err
	}
	defer conn.Close()

	logger.Info("try to connect to server", zap.String("addr", *serverAddr))

	stream, err := client.Msg(ctx)
	if err != nil {
		logger.Error("failed to communicate with server", zap.Error(err))
		return err
	}
	logger.Info("success to connect to server", zap.String("addr", *serverAddr))

	// report client version info
	data, err := proto.Marshal(&pb.ClientInfo{Os: os, Arch: arch, Version: version})
	if err != nil {
		logger.Error("failed to marshal message", zap.Error(err))
		return err
	}
	if err := stream.Send(&pb.MsgRequest{Type: pb.MsgType_Report, Data: data}); err != nil {
		logger.Error("failed to talk with server", zap.Error(err))
		return err
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			logger.Error("failed to receive message from server", zap.Error(err))
			return err
		}

		switch resp.Type {
		case pb.MsgType_Connect:
			logger.Info("server told me to spawn a new connection", zap.ByteString("addr", resp.Data))
			go connectServer(string(resp.Data))
		case pb.MsgType_WANAddr:
			logger.Info("server told me about WAN listener address", zap.ByteString("WAN listener address", resp.Data))
		default:
			logger.Error("message that current client don't support", zap.Any("msg", resp))
		}
	}
}

// Start client
func Start() {
	for {
		err := waitMsgFromServer(*serverAddr)
		errMsg := err.Error()
		if strings.Contains(errMsg, "token not valid") {
			break
		}
		time.Sleep(time.Second * 5)
	}
}
