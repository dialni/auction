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

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	ServersList    ServerList // holder øje med om servers er nede
	CurrentAuction Auction
}

type ServerList struct {
	sync.Mutex
	servers []p.AuctionService_WatcherStreamServer
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
	s := &Server{ServerPort: 4999, MaxReplicaManagers: 5}

	grpcServer := grpc.NewServer()
	var lis net.Listener
	var err error

	// Dynamically assign services to ports from 5001 to 5003 (3 active nodes, + critical zone on 5000)
	for i := 5000; i <= 5002+s.MaxReplicaManagers; i++ {
		if i == 5002+s.MaxReplicaManagers {
			fmt.Println("MaxReplicaManagers exceeded, shutting down", i, os.Getpid())
			time.Sleep(time.Duration(200) * time.Millisecond)
			os.Exit(0) // gracefully exit, instead of throwing error
		}
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
		/*
			for i := 5000; i < 5000+s.MaxReplicaManagers; i++ {
				go s.StartReplicaManager(i - 4999)
			}*/
		s.StartNetwork()
	} else {
		//go Shutdown(s.ServerPort)
	}
	//log.Println("My list of contacts:", s.clients)
	log.Printf("Server has started, listening at %v", lis.Addr())
	p.RegisterAuctionServiceServer(grpcServer, s)

	go Shutdown(s.ServerPort)
	//go s.StartAuction()

	if err = grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func Shutdown(port int) {
	time.Sleep(10 * time.Second)
	log.Printf("[%v]Server %d shutting down...\n", os.Getpid(), port)
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
	//log.Printf("Started process %d on port %d\n", process.Pid, s.ServerPort)
	process.Release()
	return
}

func (s *Server) SyncWithServers() {
	//var currentServer p.AuctionService_AuctionStreamClient
	for {
		s.CurrentAuction.Lock()
		for _, server := range s.ServersList.servers {
			//currentServer = server
			server.Send(&p.Message{
				Username: s.CurrentAuction.TopBidder,
				IsBid:    false,
				Price:    s.CurrentAuction.HighestBid,
				Kys:      false,
			})
		}
		s.CurrentAuction.Unlock()
	}
}

// grpc.BidiStreamingClient[p.Message, p.Message]
/*func (s *Server) WatcherStream(stream grpc.BidiStreamingClient[p.Message, p.Message]) error {
	for {
		msg, err := stream.Recv()
		_ = msg
		if err != nil {
			log.Println(err)
			s.ServersList.Lock()
			for i, _ := range s.ServersList.servers {
				if s.ServersList.servers[i] == stream {
					s.ServersList.servers = append(s.ServersList.servers[:i], s.ServersList.servers[i+1:]...)
					log.Printf("[%d] Server %d lost connection to a server", os.Getpid(), s.ServerPort)
					break
				}
			}
			s.ServersList.Unlock()
			//go s.StartNetwork()
			return nil
		}
	}
}*/

func (s *Server) WatcherStream(stream p.AuctionService_WatcherStreamServer) error {
	s.ServersList.Lock()
	s.ServersList.servers = append(s.ServersList.servers, stream)
	s.ServersList.Unlock()
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Println("err:", err)
			s.ServersList.Lock()
			for i, _ := range s.ServersList.servers {
				if s.ServersList.servers[i] == stream {
					s.ServersList.servers = append(s.ServersList.servers[:i], s.ServersList.servers[i+1:]...)
					log.Printf("[%d] Server %d lost connection to a server", os.Getpid(), s.ServerPort)
					break
				}
			}
			s.ServersList.Unlock()
			return nil
		}
		log.Println("msg:", msg)
		// todo: sync auction with latest
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

func (s *Server) StartNetwork() {
	s.ServersList.servers = make([]p.AuctionService_WatcherStreamServer, 0)
	var conn *grpc.ClientConn
	var err error
	for i := 5000; i < 5000+s.MaxReplicaManagers; i++ {
		if i == s.ServerPort {
			log.Println("skipping own")
			continue
		}
		log.Println("Starting ", i)
		s.StartReplicaManager(i - 5000)
		time.Sleep(time.Duration(400) * time.Millisecond)
		for {
			conn, err = grpc.NewClient(fmt.Sprintf(":%d", i), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Println("Failed to connect to server", i)
				time.Sleep(time.Duration(400) * time.Millisecond)
				continue
			}
			break
		}

		c := p.NewAuctionServiceClient(conn)
		stream, err := c.WatcherStream(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		stream.Send(&p.Message{
			Username: s.CurrentAuction.TopBidder,
			IsBid:    false,
			Price:    s.CurrentAuction.HighestBid,
			Kys:      false,
		})
		//go s.SyncWithServers()
		//go s.WatcherStream(stream)
	}
	log.Println("discovery done", s.ServersList.servers)
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
