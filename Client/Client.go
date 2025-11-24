package main

import (
	p "auction/protoc"
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"

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

	}
}

func main() {
	conn, err := grpc.NewClient(":8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	//Start listening for commands
	for {
		in.Scan()
		switch in.Text() {
		case "Bid":
			log.Println("State your bid!")
			for {
				in.Scan()
				bid, err := strconv.Atoi(in.Text())
				if err != nil {
					break
				}
				Bid(int32(bid), username)
			}
			break

		case "Result":
			Result()
			break
		}
	}
}

func Bid(bid int32, username string) {
	var msg p.Message
	msg = p.Message{
		UserID: username,
		IsBid:  true,
		Price:  bid,
	}
	stream.send(&msg)
}

func Result() {
	var msg p.Message
	msg = p.Message{
		UserID: "",
		IsBid:  false,
		Price:  0,
	}
	stream.send(&msg)
}
