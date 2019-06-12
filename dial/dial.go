package dial

import (
	"context"
	"io"
	"sync"

	"github.com/jiajunhuang/natproxy/errors"
	"github.com/jiajunhuang/natproxy/pb"
	"github.com/jiajunhuang/natproxy/pool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	logger, _ = zap.NewProduction()
)

// WithServer dial with server
func WithServer(ctx context.Context, addr string, tls bool) (pb.ServerServiceClient, *grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error
	if tls {
		conn, err = grpc.Dial(addr)
	} else {
		conn, err = grpc.Dial(addr, grpc.WithInsecure())
	}

	if err != nil {
		logger.Error("failed to connect to server server", zap.Error(err))
		return nil, nil, err
	}

	select {
	case <-ctx.Done():
		logger.Error("ctx had been done")
		return nil, nil, errors.ErrCanceled
	default:
	}

	client := pb.NewServerServiceClient(conn)
	return client, conn, nil
}

// Join two io.ReadWriteCloser and do some operations.
func Join(c1 io.ReadWriteCloser, c2 io.ReadWriteCloser) (inCount int64, outCount int64) {
	var wait sync.WaitGroup
	pipe := func(to io.ReadWriteCloser, from io.ReadWriteCloser, count *int64) {
		defer wait.Done()

		buf := pool.GetBuf(16 * 1024)
		defer pool.PutBuf(buf)
		*count, _ = io.CopyBuffer(to, from, buf)
	}

	wait.Add(2)
	go pipe(c1, c2, &inCount)
	go pipe(c2, c1, &outCount)
	wait.Wait()
	return
}
