package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/abhinandkakkadi/grpc-chat-server/chat"
	"google.golang.org/grpc"
)



func main() {

	if len(os.Args) != 3 {
		fmt.Println("Must have a url to connect to as the first argument, and a username as the second argument")
		return
	}

	ctx := context.Background()

	conn, err := grpc.Dial(os.Args[1],grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	client := chat.NewChatClient(conn)
	stream, err := client.Chat(ctx)
	if err != nil {
		panic(err)
	}


	// here the go routine will recv all the streams and the main go routine will send all the streams
	waitc := make(chan struct{})
	go func() {
		for {
				msg, err := stream.Recv()
				if err == io.EOF {
					close(waitc)
					// returning out of go routine
					return
				} else if err != nil {
					panic(err)
				}
				fmt.Println(msg.User + ": ",msg.Message)
		}
	}()

	fmt.Println("connection established, type \"quit\" or use ctrl+c to exit")
	// bufio scans whole lines of code which Scanf don't
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()
		if msg == "quit" {
			err := stream.CloseSend()
			if err != nil {
				panic(err)
			}
			break
		}

		err := stream.Send(&chat.ChatMessage{
			User: os.Args[2],
			Message: msg,
		})
		if err != nil {
			panic(err)
		}
	}
	
	<- waitc
}