package pbrpc

import (
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	pb "github.com/azcorp-cloud/pbrpc/rpc"
)

// NewClient service given connection and write calls in it
func NewClient(conn io.ReadWriteCloser) *Client {
	rand.Seed(time.Now().UnixNano())
	writeChan := make(chan *pb.Message)
	cli := &Client{
		conn:            conn,
		requestChan:     writeChan,
		m:               new(sync.RWMutex),
		responseChanMap: make(map[uint64]chan *pb.Message),
	}
	go func() {
		for {
			data, err := ioutil.ReadAll(cli.conn)
			if err != nil {
				log.Print(err)
				return
			}
			cli.handleResponse(data)
		}
	}()
	go func() {
		for request := range cli.requestChan {
			requestData, err := request.Marshal()
			if err != nil {
				log.Print(err)
				return
			}
			_, err = cli.conn.Write(requestData)
			if err != nil {
				log.Print(err)
				return
			}
		}
	}()
	return cli
}

// Client uses for calls remote procedure on server
type Client struct {
	conn            io.ReadWriteCloser
	requestChan     chan *pb.Message
	m               *sync.RWMutex
	responseChanMap map[uint64]chan *pb.Message
}

// SyncCall make a new request send it to server and wain for answer
func (c *Client) SyncCall(path string, arg proto.Marshaler, reply proto.Unmarshaler) error {
	dataArg, err := arg.Marshal()
	if err != nil {
		return err
	}
	request := &pb.Message{
		Id:     rand.Uint64(),
		Arg:    dataArg,
		Method: path,
	}
	responseChan := make(chan *pb.Message)
	c.m.Lock()
	c.responseChanMap[request.Id] = responseChan
	c.m.Unlock()
	c.requestChan <- request
	response := <-responseChan
	return reply.Unmarshal(response.Reply)
}

func (c *Client) handleResponse(data []byte) {
	response := new(pb.Message)
	err := response.Unmarshal(data)
	if err != nil {
		log.Print(err)
		return
	}
	c.m.RLock()
	responseDataChan, ok := c.responseChanMap[response.Id]
	c.m.RUnlock()
	if ok {
		responseDataChan <- response
	}
}
