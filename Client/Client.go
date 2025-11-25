package main

import (
	p "auction/protoc"
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

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
			log.Fatalf("[CLIENT]: Lost connection to server.")
		}
		log.Println(msg)

		if msg.IsDone {
			log.Printf("The auction has finished and %s won with a bid of: %d", msg.Username, msg.Price)
		} else {
			log.Printf("The auction is still ongoing and %s is the top bidder with a bid of: %d", msg.Username, msg.Price)
		}
	}
}

func main() {
	conn, err := grpc.NewClient(":5000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	c := p.NewAuctionServiceClient(conn)

	stream, err := c.AuctionStream(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Start listening for messages in the background.
	go ListenForMessages(stream)
	fmt.Println("Enter a username: ")
	in := bufio.NewScanner(os.Stdin)
	in.Scan()
	username := in.Text()
	fmt.Println("Commands available:\n - Bid\n - Result\n - Exit")
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
			Bid(int32(bid), username, stream)
			log.Println("Sending bid ")
			break

		case "result":
			Result(stream)
			break

		case "exit":
			Exit(stream)
			break
		}
	}
}

func Bid(bid int32, username string, stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	var msg p.Message
	msg = p.Message{
		Username: username,
		IsBid:    true,
		Price:    bid,
	}
	stream.Send(&msg)
}

func Result(stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	var msg p.Message
	msg = p.Message{
		Username: "",
		IsBid:    false,
		Price:    0,
	}
	stream.Send(&msg)
}

func Exit(stream grpc.BidiStreamingClient[p.Message, p.Result]) {
	var msg p.Message
	msg = p.Message{
		Username: "",
		IsBid:    false,
		Price:    0,
		Kys:      true,
	}
	stream.Send(&msg)
}
