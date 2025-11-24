package main

import (
	p "auction/protoc"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
)

const (
	SERVER int = iota
	CLIENT
)

type Server struct {
	p.UnimplementedAuctionServiceServer
	//clients map[int]*grpc.ServerStream // måske useless, hvis der ikke er en grund til at holde øje med dem
	servers        map[int]*p.AuctionService_AuctionStreamClient // holder øje med om servers er nede
	CurrentAuction Auction
}

type Auction struct {
	sync.Mutex
	TopBidder  int   // port of highest bidder
	HighestBid int32 // current highest bid
	TimeLeft   int64 // time left before auction ends
	IsDone     bool  // if auction is done
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

func (s *Server) StartAuction() {
	var auctionTime int64 = 60
	s.CurrentAuction = Auction{TopBidder: 0, HighestBid: 10, TimeLeft: time.Now().Unix() + auctionTime}
	s.WaitForDone()
	s.CurrentAuction.Lock()
	s.CurrentAuction.IsDone = true
	s.CurrentAuction.Unlock()
}

func (s *Server) StartProcess() {}

func (s *Server) Watcher() {}

func (s *Server) WaitForDone() {
	for {
		if s.CurrentAuction.TimeLeft < time.Now().Unix() {
			return
		}
	}
}

// AuctionStream is the server-to-server connection stream.
func (s *Server) AuctionStream(stream p.AuctionService_AuctionStreamServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			log.Println("AuctionStream Recv error:", err)
		}
		s.CurrentAuction.Lock()
		if req.IsBid {
			if s.CurrentAuction.HighestBid < *req.Price {
				s.CurrentAuction.HighestBid = *req.Price //update highest bidder (max is int32 limit)
			}
		} else {
			if s.CurrentAuction.IsDone {
				//spit out winner
			} else {
				//spit out highest bid
			}

		}
		s.CurrentAuction.Unlock()

	}
	return nil
}
