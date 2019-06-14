package dial

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"log"
	"sync"

	"github.com/jiajunhuang/natproxy/errors"
	"github.com/jiajunhuang/natproxy/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	socketBufferSize = flag.Int("socketBufferSize", 1024*32, "连接缓冲区大小，越大越快，但是也更吃内存")
)

// WithServer dial with server
func WithServer(ctx context.Context, addr string, useTLS bool) (pb.ServerServiceClient, *grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error
	if useTLS {
		creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
		conn, err = grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	} else {
		conn, err = grpc.Dial(addr, grpc.WithInsecure())
	}

	if err != nil {
		log.Printf("failed to connect to server server: %s", err)
		return nil, nil, err
	}

	select {
	case <-ctx.Done():
		log.Printf("ctx had been done, so give up to dial with server")
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
		defer c1.Close()
		defer c2.Close()
		defer wait.Done()

		buf := make([]byte, *socketBufferSize)

		*count, _ = io.CopyBuffer(to, from, buf)
	}

	wait.Add(2)
	go pipe(c1, c2, &inCount)
	go pipe(c2, c1, &outCount)
	wait.Wait()
	return
}
