package server

import (
	"log"
	"net"

	"github.com/jiajunhuang/natproxy/dial"
	"github.com/jiajunhuang/natproxy/pb"
	"google.golang.org/grpc/peer"
)

type manager struct {
	service      *service
	wanConnCh    chan net.Conn        // connections from WAN
	clientConnCh chan net.Conn        // connections from client
	msgCh        chan *pb.MsgResponse // messages send to client
	clientMsgCh  chan *pb.MsgRequest  // messages from client
}

func newManager(svc *service, bufSize int) *manager {
	return &manager{
		service:      svc,
		wanConnCh:    make(chan net.Conn, bufSize),
		clientConnCh: make(chan net.Conn, bufSize),
		msgCh:        make(chan *pb.MsgResponse, bufSize),
		clientMsgCh:  make(chan *pb.MsgRequest, bufSize),
	}
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
				log.Printf("failed to receive connection from client connection channel")
				return
			}
			wanConnAddr, clientConnAddr := wanConn.LocalAddr(), clientConn.RemoteAddr()
			defer log.Printf("connection between WAN(%s) & client(%s) disconnected", wanConnAddr, clientConnAddr)

			// 把两个connection串起来
			dial.Join(wanConn, clientConn)
		}()
	}
}

// 接收来自客户端的请求并且传递给channel
func (manager *manager) receiveConnFromClient(client *peer.Peer, clientListener net.Listener) {
	defer close(manager.clientConnCh)

	log.Printf("start to wait new connections from client...")
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

	log.Printf("start to wait new connections from WAN...")
	for {
		conn, err := wanListener.Accept()
		if err != nil {
			return
		}
		manager.wanConnCh <- conn
	}
}
