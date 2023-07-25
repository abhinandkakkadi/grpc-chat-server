package main

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/abhinandkakkadi/grpc-chat-server/chat"
	"google.golang.org/grpc"
)

// in order to have the thread safety we are doing the below thing

type Connection struct {
	conn chat.Chat_ChatServer
	// send channel - required to send msg from send function to start through channels
	send chan *chat.ChatMessage
	quit chan struct{}
}

// new connection takes in a stream from which data can be received and send back
func NewConnection(conn chat.Chat_ChatServer) *Connection {
	c := &Connection{
		conn: conn,
		send: make(chan *chat.ChatMessage),
		quit: make(chan struct{}),
	}
	go c.start()
	return c
}

func (c *Connection) Close() error {
	close(c.quit)
	close(c.send)
	return nil
}

func (c *Connection) Send(msg *chat.ChatMessage) {

	defer func() {
		// Ignore any errors about sending on a closed channel
		recover()
	}()
	// iif send got close due to the connection getting closed, the errors will be ignorder
	// for trying to send a message to a closed channel
	c.send <- msg

}

func (c *Connection) start() {

	running := true
	for running {
		select {
		case msg := <- c.send:
			// this send is not the one defined here. But the one used to send data to client
			c.conn.Send(msg)
		case <- c.quit:
			running = false
		}
	}
}

// send only channel
func (c *Connection) GetMessages(broadcast chan<- *chat.ChatMessage) error {

	for {
		msg, err := c.conn.Recv()
		if err == io.EOF {
			c.Close()
			return nil
		} else if err != nil {
			c.Close()
			return err
		}
		go func(msg *chat.ChatMessage) {
			select {
			case broadcast <- msg:
			case <- c.quit:
			}
		}(msg)
	}
}

type ChatServer struct {
	// broadcast channel is the one to which the connection will send their messages to
	// and it will broadcast to all connections 
	broadcast chan *chat.ChatMessage
	// as all the connection has it's own quit channel, it also have a global wide quit channel 
	quit chan struct{}
	connections []*Connection
	// when erver we are using the []*Connection we will be locking the value using mutex
	connLock sync.Mutex
}

func NewChatServer() *ChatServer {
	srv := &ChatServer{
		broadcast: make(chan *chat.ChatMessage),
		quit: make(chan struct{}),
		// we don't have to initialize the connection as it can be created at time of appending
		// we don't have to create sync Mutex as zero value is valid for it
	}
	go srv.start()
	return srv
}

func (c *ChatServer) Close() error {
	close(c.quit)
	return nil
}

func (c *ChatServer) start() {

	running := true
	for running {
		select {
		case msg := <- c.broadcast:
			// if a message is coming through the broadcast channel, lock the connection lock 
			// iterate through the connection and send the broadcast message in a go routine safe way
			c.connLock.Lock()
			for _,v := range c.connections {
				// here we are using go routine so that the server will not be blocked if one of the connection 
				// is being slow
				go v.Send(msg)
			}
			c.connLock.Unlock()
		case <- c.quit:
			running = false
		}
	}
}

func (c *ChatServer) Chat(stream chat.Chat_ChatServer) error {

	conn := NewConnection(stream)

	// It locks the connection lock and add the connections to the connection slice 
	c.connLock.Lock()
	c.connections = append(c.connections, conn)
	c.connLock.Unlock()

	// inside this function the code to get the message from user have been written
	err := conn.GetMessages(c.broadcast)
	// when GetMessages returns that means that connection is done

	c.connLock.Lock()
	// loop through the connection, find our connection and remove it from the slice of connections
	for i,v := range c.connections {
		if v == conn {
			c.connections = append(c.connections[:i],c.connections[i+1:]...)
		}
	}

	c.connLock.Unlock()

	return err

}

func main() {

	lst, err := net.Listen("tcp",":8080")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()

	srv := NewChatServer()
	chat.RegisterChatServer(s,srv)

	fmt.Println("Now serving at port 8080")
	err = s.Serve(lst)
	if err != nil {
		panic(err)
	}
}
