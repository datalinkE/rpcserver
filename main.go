package main

import (
	"errors"
	"github.com/datalinkE/jsonrpc-sample/method/methodcodec"
	"github.com/datalinkE/jsonrpc-sample/method/methodserver"
	"github.com/gorilla/rpc/v2"
	jsonrpc "github.com/gorilla/rpc/v2/json2"
	"gopkg.in/gin-gonic/gin.v1"
	"log"
	"net/http"
)

type Args struct {
	A, B int
}

type Quotient struct {
	Quo, Rem int
}

type Arith int

func (t *Arith) Multiply(r *http.Request, args *Args, reply *int) error {
	log.Print("multiply")
	*reply = args.A * args.B
	return nil
}

func (t *Arith) Divide(r *http.Request, args *Args, quo *Quotient) error {
	log.Print("divide")
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}

func main() {
	log.Print("main")
	arith := new(Arith)
	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(methodcodec.NewMethodCodec(), "application/json")
	rpcServer.RegisterService(arith, "")

	methodserver := methodserver.NewServer()
	methodserver.RegisterCodec(jsonrpc.NewCodec(), "application/json")
	methodserver.RegisterService(arith, "")

	router := gin.Default()
	router.POST("/jsonrpc/v1/:method", gin.WrapH(rpcServer))
	router.POST("/jsonrpc/v2/:method", gin.WrapH(methodserver))

	log.Fatal(router.Run())
}
