package main

import (
	p "auction/protoc"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Server struct {
	p.UnimplementedAuctionServiceServer
	ServerPort     int
	clients        []p.AuctionService_WatcherStreamServer
	CurrentAuction Auction
	kys            bool
}

type Auction struct {
	ID         int32  // auction id
	TopBidder  string // port of highest bidder
	HighestBid int32  // current highest bid
	TimeStart  int64  // open for bids
	TimeLeft   int64
	IsDone     bool // if auction is done
}

func main() {
	s := &Server{kys: false, CurrentAuction: Auction{
		ID:         0,
		TopBidder:  "",
		HighestBid: 0,
		TimeStart:  0,
		TimeLeft:   9999999999999999,
		IsDone:     false,
	}}

	// Assign port on startup
	s.ServerPort = 5000
	lis, err := net.Listen("tcp", ":5000")
	if err != nil {
		s.ServerPort = 5001
		lis, err = net.Listen("tcp", ":5001")
		if err != nil {
			s.ServerPort = 5002
			lis, err = net.Listen("tcp", ":5002")
			if err != nil {
				//log.Fatal(err)
				log.Println("exiting server")
				os.Exit(0)
			}
		}
	}
	grpcServer := grpc.NewServer()
	log.Printf("Server has started, listening at %v", lis.Addr())
	p.RegisterAuctionServiceServer(grpcServer, s)
	if s.ServerPort == 5000 {
		s.CurrentAuction = Auction{
			ID:         1,
			TopBidder:  "",
			HighestBid: 10,
			TimeStart:  time.Now().Unix(),
			TimeLeft:   time.Now().Unix() + 30,
			IsDone:     false,
		}
		go s.SpawnProcess()
		time.Sleep(time.Duration(500) * time.Millisecond)
		go s.SpawnProcess()
		time.Sleep(time.Duration(500) * time.Millisecond)
		go s.RunAuction()
	}
	time.Sleep(time.Duration(1000) * time.Millisecond)
	go s.StartNetwork()
	time.Sleep(time.Duration(1000) * time.Millisecond)
	go s.Sync()
	go s.Shutdown()

	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (s *Server) Shutdown() {
	if s.ServerPort == 5001 {
		time.Sleep(time.Duration(10000) * time.Millisecond)
		log.Println("Server is shutting down")
		os.Exit(0)
	}
}

// WaitForDone Constantly check if auction is over
func (s *Server) WaitForDone() {
	for {
		if s.CurrentAuction.TimeLeft < time.Now().Unix() {
			return
		}
	}
}

func (s *Server) WaitForStart() {
	for {
		if s.CurrentAuction.TimeStart < time.Now().Unix() {
			return
		}
	}
}

func (s *Server) RunAuction() {
	log.Println("Starting Auction")
	var auc Auction
	var timeBeforeNextAuc int64 = 5
	var aucDuration int64 = 5
	for {
		s.WaitForStart()
		s.CurrentAuction.IsDone = false
		log.Println("Auction started")

		s.WaitForDone()
		if s.ServerPort == 5000 {
			auc = Auction{
				ID:         s.CurrentAuction.ID + 1,
				TopBidder:  "",
				HighestBid: 10,
				TimeStart:  time.Now().Unix() + timeBeforeNextAuc,
				TimeLeft:   time.Now().Unix() + timeBeforeNextAuc + aucDuration,
				IsDone:     true,
			}
			s.CurrentAuction = auc
		}
		log.Println("Auction finished")
	}
}

func (s *Server) StartNetwork() {
	time.Sleep(1000 * time.Millisecond)
	s.clients = make([]p.AuctionService_WatcherStreamServer, 0)
	var conn *grpc.ClientConn
	var err error
	for i := 5000; i <= 5002; i++ {
		if i == s.ServerPort {
			//log.Println("skipping own", i)
			continue
		}
		conn, err = grpc.NewClient(fmt.Sprintf(":%d", i), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("failed to connect: %v", err)
		}
		c := p.NewAuctionServiceClient(conn)
		stream, err := c.WatcherStream(context.Background())
		if err != nil {
			log.Fatalf("failed to connect: %v", err)
		}
		if s.ServerPort == 5001 {
			log.Println("sending msg from 5001 2")
		}
		go s.ListenForMessages(stream)
		go stream.Send(&p.SyncMessage{
			Username:  s.CurrentAuction.TopBidder,
			Price:     s.CurrentAuction.HighestBid,
			Kys:       s.kys,
			AuctionID: s.CurrentAuction.ID,
			TimeStart: s.CurrentAuction.TimeStart,
			TimeLeft:  s.CurrentAuction.TimeLeft,
			IsDone:    s.CurrentAuction.IsDone,
			IsJoin:    true,
		})
	}
}

func (s *Server) SpawnProcess() {
	time.Sleep(time.Duration(300) * time.Millisecond)
	cmd, err := exec.LookPath("./server")
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
	process.Release()
}

func (s *Server) Sync() {
	log.Printf("Server is syncing from %d", s.ServerPort)
	for {
		if s.ServerPort == 5001 {
			log.Println(s.clients)
		}
		time.Sleep(time.Duration(1000) * time.Millisecond)

		for _, client := range s.clients {
			// attempt at recover function if things go wrong in the loop
			if r := recover(); r != nil {
			}
			if s.ServerPort == 5001 {
				log.Println("sending msg from", s.ServerPort)
			}
			go client.Send(&p.SyncMessage{
				Username:  s.CurrentAuction.TopBidder,
				Price:     s.CurrentAuction.HighestBid,
				Kys:       s.kys,
				AuctionID: s.CurrentAuction.ID,
				TimeStart: s.CurrentAuction.TimeStart,
				TimeLeft:  s.CurrentAuction.TimeLeft,
				IsDone:    s.CurrentAuction.IsDone,
				IsJoin:    false,
			})
		}
	}
}

// Sync listener
func (s *Server) ListenForMessages(stream grpc.BidiStreamingClient[p.SyncMessage, p.SyncMessage]) {
	for {
		msg, err := stream.Recv()

		if err != nil {
			log.Printf("[%d]: Server %d lost connection to a client.", os.Getpid(), s.ServerPort)
			go s.SpawnProcess()
			time.Sleep(time.Duration(2500) * time.Millisecond)
			go s.StartNetwork()
			return
		}
		if msg.IsJoin {
			log.Printf("[%d]: Server %d got join request.", os.Getpid(), s.ServerPort)
			go stream.Send(&p.SyncMessage{
				Username:  s.CurrentAuction.TopBidder,
				Price:     s.CurrentAuction.HighestBid,
				Kys:       s.kys,
				AuctionID: s.CurrentAuction.ID,
				TimeStart: s.CurrentAuction.TimeStart,
				TimeLeft:  s.CurrentAuction.TimeLeft,
				IsDone:    s.CurrentAuction.IsDone,
				IsJoin:    false,
			})
		}

		//log.Printf("%d got msg: %s", s.ServerPort, msg)
		if s.CurrentAuction.ID < msg.AuctionID || s.CurrentAuction.HighestBid < msg.Price {
			log.Printf("[%d] Server %d updated their latest auction from sync message: %v", os.Getpid(), s.ServerPort, msg)
			s.CurrentAuction = Auction{
				ID:         msg.AuctionID,
				TopBidder:  msg.Username,
				HighestBid: msg.Price,
				TimeStart:  msg.TimeStart,
				TimeLeft:   msg.TimeLeft,
				IsDone:     msg.IsDone,
			}
			log.Printf("[%d] Server %d has this as latest: %v", os.Getpid(), s.ServerPort, s.CurrentAuction)
		}
	}
}

func (s *Server) WatcherStream(stream p.AuctionService_WatcherStreamServer) error {
	log.Println("Server received a new stream")
	s.clients = append(s.clients, stream)

	log.Println(s.ServerPort, s.clients)
	for {
		_, err := stream.Recv()
		if err != nil {
			return nil
			//log.Printf("[%d]: Server %d lost connection to a client.", os.Getpid(), s.ServerPort)
		}
	}
}
