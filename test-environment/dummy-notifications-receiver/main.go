package main

import (
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"os"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/keypb"
)

type Server struct {
	keypb.UnimplementedKeyPushServiceServer
}

func (s *Server) SendKeyDiff(stream keypb.KeyPushService_SendKeyDiffServer) error {
	reqCh := make(chan *keypb.SendKeyDiffRequest)
	keyDiffCh := keypb.ProcessSendKeyDiffRequests(reqCh)
	
	err := func() error {
		defer close(reqCh)
		for {
			req, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
	
			select {
			case result := <-keyDiffCh:
				code := codes.Unknown
				if keypb.IsApiContractError(result.Error) {
					code = codes.InvalidArgument
				}
				return status.New(code, result.Error.Error()).Err()
			default:
			}
	
			reqCh <- req
		}

		return nil
	}()

	if err != nil {
		return err
	}

	result := <-keyDiffCh
	if result.Error != nil {
		code := codes.Unknown
		if keypb.IsApiContractError(result.Error) {
			code = codes.InvalidArgument
		}
		return status.New(code, result.Error.Error()).Err()
	}

	fmt.Printf("Received: %v\n", result.KeyDiff)

	sendErr := stream.SendAndClose(&keypb.SendKeyDiffResponse{})
	return sendErr
}

func main() {
	var grpcServer *grpc.Server

	listener, err := net.Listen("tcp", "127.0.0.10:8080")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	grpcServer = grpc.NewServer()
	keypb.RegisterKeyPushServiceServer(grpcServer, &Server{})
	fmt.Println("Started listening")
	serveErr := grpcServer.Serve(listener)
	fmt.Println("Stopped listening")
	if serveErr != nil {
		fmt.Println(serveErr)
		os.Exit(1)
	}
}