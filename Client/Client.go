package main

import (
	p "auction/protoc"
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// todo: change log messages to correct form
// ListenForMessages is mostly self-explanitory, it listens are parses messages from the server to the clients terminal
func ListenForMessages(stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			//log.Println("Lost connection to server, retrying,", err)
			IsServerDown = true
			time.Sleep(time.Duration(2500) * time.Millisecond)
			go StartConnection()
			return
		}
		log.Println(msg)

		if msg.Success {
			log.Println("Successfully executed command.")
		} else {
			log.Println("Failed to execute command, check auction using Query.")
		}
	}
}

var conn *grpc.ClientConn
var stream grpc.BidiStreamingClient[p.Message, p.Result]
var err error
var IsServerDown bool

func StartConnection() {
	conn, err = grpc.NewClient(":5000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		IsServerDown = true
		log.Println("Failed to connect to server, retrying in 2s...")
		time.Sleep(time.Duration(2) * time.Second)
		go StartConnection()
		return
	}

	c := p.NewAuctionServiceClient(conn)

	stream, err = c.AuctionStream(context.Background())
	if err != nil {
		IsServerDown = true
		log.Println("Failed to connect to server, retrying in 3s...")
		time.Sleep(time.Duration(3) * time.Second)
		go StartConnection()
		return
	}

	go ListenForMessages(stream)
	if IsServerDown {
		log.Println("Regained connection to server.")
		IsServerDown = false
	}
	// Start listening for messages in the background.

}

func main() {

	StartConnection()
	fmt.Println("Enter a username: ")
	in := bufio.NewScanner(os.Stdin)
	in.Scan()
	username := in.Text()
	fmt.Println("Commands available:\n - Bid\n - Query\n - Exit")
	//Start listening for commands
	for {
		in.Scan()
		var command = strings.ToLower(in.Text())
		switch command {
		case "bid":
			log.Println("State your bid!")
			in.Scan()
			bid, err := strconv.Atoi(in.Text())
			if err != nil || bid < 1 {
				log.Println("Enter a valid bid, use only positive integers.")
				continue
			}
			if IsServerDown {
				for {
					time.Sleep(time.Duration(100) * time.Millisecond)
					if !IsServerDown {
						log.Println("Server is up again.")
						break
					}
				}
			}
			time.Sleep(time.Duration(200) * time.Millisecond)
			Bid(int32(bid), username, stream)
			log.Println("Sending bid ")
			break

		case "query":
			Query(stream)
			break

		case "exit":
			Exit(stream)
			break
		default:
			fmt.Println("Unknown command, please try again")
			break
		}

	}
}

func Bid(bid int32, username string, stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	var msg p.Message
	msg = p.Message{
		Username:  username,
		IsBid:     true,
		Price:     bid,
		Kys:       false,
		AuctionID: 0,
	}
	// wait until server returns

	log.Println("bid sent")
	stream.Send(&msg)
}

func Query(stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	var msg p.Message
	msg = p.Message{
		Username:  "",
		IsBid:     false,
		Price:     0,
		Kys:       false,
		AuctionID: 0,
	} // wait until server returns
	if IsServerDown {
		for {
			if !IsServerDown {
				break
			}
		}
	}
	stream.Send(&msg)
}

func Exit(stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	var msg p.Message
	msg = p.Message{
		Username:  "",
		IsBid:     false,
		Price:     0,
		Kys:       true,
		AuctionID: 0,
	}
	// wait until server returns
	if IsServerDown {
		for {
			if !IsServerDown {
				break
			}
		}
	}
	stream.Send(&msg)
}
