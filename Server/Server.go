package main

import (
	p "auction/protoc"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
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
	ServerPort         int
	MaxReplicaManagers int
	//clients map[int]*grpc.ServerStream // måske useless, hvis der ikke er en grund til at holde øje med dem
	servers        map[int]*p.AuctionService_AuctionStreamClient // holder øje med om servers er nede
	CurrentAuction Auction
}

type Auction struct {
	sync.Mutex
	TopBidder  string // port of highest bidder
	HighestBid int32  // current highest bid
	TimeLeft   int64  // time left before auction ends
	IsDone     bool   // if auction is done
}

type BrokerMessage struct {
}

func main() {
	s := &Server{ServerPort: 4999, MaxReplicaManagers: 3}

	grpcServer := grpc.NewServer()
	var lis net.Listener
	var err error

	// Dynamically assign services to ports from 5001 to 5003 (3 active nodes, + critical zone on 5000)
	for i := 5000; i < 5001+s.MaxReplicaManagers; i++ {
		s.ServerPort++
		lis, err = net.Listen("tcp", fmt.Sprintf(":%d", s.ServerPort))
		if err != nil {
			//log.Println("Port ", s.ServerPort, " in use, trying next port in line...")
			continue
		} else {
			break
		}
	}
	if s.ServerPort == 5000 {
		for i := 5000; i < 5000+s.MaxReplicaManagers; i++ {
			go s.StartReplicaManager(i - 4999)
		}
	} else {
		go Shutdown(s.ServerPort)
	}
	//log.Println("My list of contacts:", s.clients)
	log.Printf("Server has started, listening at %v", lis.Addr())
	p.RegisterAuctionServiceServer(grpcServer, s)

	go s.StartAuction()

	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func Shutdown(port int) {
	time.Sleep(5 * time.Second)
	log.Printf("Server %d shutting down...\n", port)
	os.Exit(0)
}

func (s *Server) StartAuction() {
	var auctionTime int64 = 60
	//start an auction every 15 seconds as long as there are no ongoing ones
	for {
		log.Println("Starting Auction, 60 seconds to bid")
		s.CurrentAuction = Auction{TopBidder: "", HighestBid: 10, TimeLeft: time.Now().Unix() + auctionTime}
		s.WaitForDone()
		s.CurrentAuction.Lock()
		s.CurrentAuction.IsDone = true
		s.CurrentAuction.Unlock()

		//wait for a while
		log.Println("Starting Auction in 15 seconds.")
		time.Sleep(time.Duration(15) * time.Second)
	}
}

func (s *Server) StartReplicaManager(offset int) {
	time.Sleep(time.Duration(300*offset) * time.Millisecond)
	cmd, err := exec.LookPath("./Server")
	if err != nil {
		log.Fatalf("Server is not installed.")
	}
	attr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}
	process, err := os.StartProcess(cmd, []string{cmd}, attr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	log.Printf("Started process %d on port %d\n", process.Pid, s.ServerPort)
	process.Release()
	return
}

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
	var msg p.Result
	for {
		req, err := stream.Recv()
		if err != nil {
			log.Println("Client disconnected")
			return nil
		}
		log.Printf("Received Auction Request: %v", req)

		if rand.Intn(20)+1 == 20 {
			fmt.Printf("OHHHH Nooo im crashing :(((")
			os.Exit(0)
		}

		s.CurrentAuction.Lock()
		if req.Kys {
			//send to all other servers
			os.Exit(0)
		}

		if req.IsBid {
			if !s.CurrentAuction.IsDone {
				if s.CurrentAuction.HighestBid < req.Price {
					s.CurrentAuction.HighestBid = req.Price   //update highest bidder (max is int32 limit)
					s.CurrentAuction.TopBidder = req.Username //update top bidder username
				}
			} else {
				break
			}
		} else {
			if s.CurrentAuction.IsDone {
				//spit out winner
				msg = p.Result{
					Success:  true,
					IsDone:   true,
					Username: s.CurrentAuction.TopBidder,
					Price:    s.CurrentAuction.HighestBid,
					TimeLeft: 0,
				}
				stream.Send(&msg)
			} else {
				//spit out highest bid
				msg = p.Result{
					Success:  true,
					Username: s.CurrentAuction.TopBidder,
					Price:    s.CurrentAuction.HighestBid,
					TimeLeft: s.CurrentAuction.TimeLeft,
					IsDone:   false,
				}
				stream.Send(&msg)
			}
		}
		s.CurrentAuction.Unlock()

	}
	return nil
}
