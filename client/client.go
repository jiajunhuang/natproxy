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
	version = "0.0.3"
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
		logger.Error("无法连接服务器", zap.String("服务器地址", addr), zap.Error(err))
		return
	}

	localConn, err := net.Dial("tcp", *localAddr)
	if err != nil {
		logger.Error("无法连接本地目标地址", zap.String("本地地址", *localAddr), zap.Error(err))
		return
	}

	dial.Join(conn, localConn)
}

func waitMsgFromServer(addr string) error {
	md := metadata.Pairs("natrp-token", *token)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client, conn, err := dial.WithServer(ctx, *serverAddr, *useTLS)
	if err != nil {
		logger.Error("无法连接服务器", zap.Error(err))
		return err
	}
	defer conn.Close()

	logger.Info("准备连接到服务器", zap.String("服务器地址", *serverAddr))

	stream, err := client.Msg(ctx)
	if err != nil {
		logger.Error("无法与服务器通信", zap.Error(err))
		return err
	}
	logger.Info("成功连接到服务器", zap.String("服务器地址", *serverAddr))

	// report client version info
	data, err := proto.Marshal(&pb.ClientInfo{Os: os, Arch: arch, Version: version})
	if err != nil {
		logger.Error("无法压缩信息", zap.Error(err))
		return err
	}
	if err := stream.Send(&pb.MsgRequest{Type: pb.MsgType_Report, Data: data}); err != nil {
		logger.Error("无法发送消息到服务器", zap.Error(err))
		return err
	}

	for {
		resp, err := stream.Recv()
		if err != nil {
			logger.Error("无法从服务器接收消息", zap.Error(err))
			return err
		}

		switch resp.Type {
		case pb.MsgType_Connect:
			logger.Info("服务器要求发起新连接", zap.ByteString("目标地址", resp.Data))
			go connectServer(string(resp.Data))
		case pb.MsgType_WANAddr:
			logger.Info("服务器分配的公网地址是", zap.ByteString("公网地址", resp.Data))
		default:
			logger.Error("当前版本客户端不支持本消息，请升级", zap.Any("消息", resp))
		}
	}
}

// Start client
func Start() {
	for {
		err := waitMsgFromServer(*serverAddr)
		errMsg := err.Error()
		if strings.Contains(errMsg, "token not valid") {
			logger.Error("您的token不对，请检查是否正确配置，参考：https://jiajunhuang.com/natproxy")
			break
		}
		time.Sleep(time.Second * 5)
	}
}
