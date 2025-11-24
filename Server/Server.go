package main

import (
	p "auction/protoc"
	"io"
	"log"
	"net"

	"google.golang.org/grpc"
)

const (
	SERVER int = iota
	CLIENT
)

type Server struct {
	p.UnimplementedAuctionServiceServer
	clients map[int]*grpc.ServerStream
	servers map[int]*grpc.ClientStream
}

type BrokerMessage struct {

}

func main() {
	s := &Server{}
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	p.RegisterAuctionServiceServer(grpcServer, s)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (s* Server) StartProcess(){}
func (s* Server) Watcher(){}

// AuctionStream is the server-to-server connection stream.
func (s* Server) AuctionStream(stream p.AuctionService_AuctionStreamServer) error {
	for{
		req, err := stream.Recv()
		if err != nil {
			log.Println("AuctionStream Recv error:", err)
		}


	}
	return nil
}