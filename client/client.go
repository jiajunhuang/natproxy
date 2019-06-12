package client

import (
	"context"
	"flag"
	"time"

	"github.com/jiajunhuang/natproxy/dial"
	"github.com/jiajunhuang/natproxy/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

var (
	logger, _ = zap.NewProduction()

	localAddr  = flag.String("local", "127.0.0.1:80", "-local=<你本地需要转发的地址>")
	serverAddr = flag.String("server", "127.0.0.1:10020", "-server=<你的服务器地址>")
	token      = flag.String("token", "balalaxiaomoxian", "-token=<你的token>")
)

func waitMsgFromServer(addr string) {
	md := metadata.Pairs("natrp-token", *token)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client, conn, err := dial.WithServer(ctx, *serverAddr, false)
	if err != nil {
		logger.Error("failed to connect to server server", zap.Error(err))
		return
	}
	defer conn.Close()

	logger.Info("try to connect to server", zap.String("addr", *serverAddr))

	stream, err := client.Msg(ctx)
	if err != nil {
		logger.Error("failed to communicate with server", zap.Error(err))
		return
	}

	logger.Info("success to connect to server", zap.String("addr", *serverAddr))

	for {
		resp, err := stream.Recv()
		if err != nil {
			logger.Error("failed to receive message from server", zap.Error(err))
			return
		}

		switch resp.Type {
		case pb.MsgType_Connect:
			logger.Info("server told me to spawn a new connection", zap.ByteString("remote", resp.Data))
		case pb.MsgType_WANAddr:
			logger.Info("server told me about WAN listener address", zap.ByteString("WAN listener address", resp.Data))
		case pb.MsgType_ClientConnAddr:
			logger.Info("server told me about client listener address", zap.ByteString("client listener address", resp.Data))
		default:
			logger.Error("message that current client don't support", zap.Any("msg", resp))
		}
	}
}

// Start client
func Start() {
	for {
		waitMsgFromServer(*serverAddr)
		time.Sleep(time.Second * 10)
	}
}
