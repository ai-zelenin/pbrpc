# Protobuf RPC server over io.ReadWriteCloser(websocket,tcp)
## Usage
Make proto file
```protobuf
syntax = "proto3";
package bench;

service Bench {
    rpc Echo (BenchArg) returns (BenchReply);
    rpc EchoStream (BenchArg) returns (stream BenchReply);
    rpc EchoBiStream (stream BenchArg) returns (stream BenchReply);
}

message BenchArg {
    string val = 1;
}
message BenchReply {
    string val = 1;
}
```
Generate  frontend(typescript) and backend(golang,c++) source files
```bash
function gen-go() {
    ${PROTOC} -I "${PROTOC_INCLUDE}" \
    -I . \
    -I ${GOPATH}/src \
    -I ${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
    --gofast_out=plugins=grpc:"$2" "$1"
}
function gen-cpp() {
    ${PROTOC} -I "${PROTOC_INCLUDE}" \
    -I . \
    -I ${GOPATH}/src \
    -I ${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
    --cpp_out=:"$2" "$1"
}
function gen-ts() {
    ${PROTOC} -I "${PROTOC_INCLUDE}" \
    -I . \
    -I ${GOPATH}/src \
    -I ${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
    --js_out=import_style=commonjs,binary:"$2" \
    --plugin=protoc-gen-ts="$HOME"/.npm-global/bin/protoc-gen-ts --ts_out=service=true:"$2" "$1"
}
```
Serve websocket conn
```go
var protoRpcServer *pbrpc.Server

func main() {
	wg := &sync.WaitGroup{}
	router := mux.NewRouter()
	router.HandleFunc("/ws", DealingWebsocket)

	grpcServer := grpc.NewServer()
	bench.RegisterBenchServer(grpcServer, new(BenchService))
    protoRpcServer = pbrpc.NewServer(grpcServer)
    
	wg.Add(1)
	http.ListenAndServe(":8080", router)
	wg.Done()

	wg.Wait()
}

func DealingWebsocket(ws *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	protoRpcServer.ServeConnCtx(ctx, ws)
}

type BenchService struct {
}

func (bs *BenchService) EchoStream(arg *bench.BenchArg, replyStream bench.Bench_EchoStreamServer) error {
	log.Print(arg.Val)
	for i := 0; i < 10; i++ {
		select {
		case <-replyStream.Context().Done():
			return replyStream.Context().Err()
		default:
			reply := &bench.BenchReply{Val: fmt.Sprintf("%d", i)}
			replyStream.Send(reply)
			time.Sleep(time.Second * 1)
			log.Print(*reply)
		}
	}
	return nil
}

func (bs *BenchService) EchoBiStream(biDirectStream bench.Bench_EchoBiStreamServer) error {
	for i := 0; i < 10; i++ {
		benchArg, err := biDirectStream.Recv()
		if err != nil {
			return err
		}
		benchReply := &bench.BenchReply{Val: benchArg.Val}
		err = biDirectStream.Send(benchReply)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bs *BenchService) Echo(ctx context.Context, arg *bench.BenchArg) (*bench.BenchReply, error) {
	return &bench.BenchReply{
		Val: arg.Val,
	}, nil
}
```
Call remote method with golang client 
```go
origin := "http://localhost/"
url := "ws://localhost:8080/ws"
ws, err := websocket.Dial(url, "", origin)
if err != nil {
    log.Fatal(err)
}
	client := pbrpc.NewClient(ws)
	for j := 0; j < m; j++ {
		wg.Add(1)
		go func() {
			for i := 0; i < n; i++ {
				arg := &bench.BenchArg{
					Val: "echo!",
				}
				reply := &bench.BenchReply{}
				err := client.SyncCall("Bench.Echo", arg, reply)
				if err != nil {
					log.Print(err)
				}
				if arg.Val != reply.Val {
					log.Print("not equal")
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
```
