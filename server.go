// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Copyright 2017 Andrey Pichugin. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpcserver

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// ----------------------------------------------------------------------------
// Server
// ----------------------------------------------------------------------------

// NewServer returns a new RPC server.
//
// Methods from the receiver will be extracted if these rules are satisfied:
//
//    - The receiver is exported (begins with an upper case letter) or local
//      (defined in the package registering the service).
//    - The method name is exported.
//    - The method has three arguments: *http.Request, *args, *reply.
//    - All three arguments are pointers.
//    - The second and third arguments are exported or local.
//    - The method has return type error.
//

func NewServer(receiver interface{}) (*Server, error) {
	service, err := NewRpcService(receiver)
	if err != nil {
		return nil, err
	}

	server := &Server{
		codecs:  make(map[string]Codec),
		service: service,
	}
	// TODO: maybe register default json-rpc codec
	return server, nil
}

// Server serves registered RPC service using registered codecs.
type Server struct {
	codecs  map[string]Codec
	service *RpcService
}

// RegisterCodec adds a new codec to the server.
//
// Codecs are defined to process a given serialization scheme, e.g., JSON or
// XML. A codec is chosen based on the "Content-Type" header from the request,
// excluding the charset definition.
func (s *Server) RegisterCodec(codec Codec, contentType string) {
	s.codecs[strings.ToLower(contentType)] = codec
}

// HasMethod returns true if the given method is registered.
//
// The method uses a dotted notation as in "Service.Method".
func (s *Server) HasMethod(method string) bool {
	if _, err := s.service.Get(method); err == nil {
		return true
	}
	return false
}

// ServeHTTP
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		WriteError(w, 405, "rpc: POST method required, received "+r.Method)
		return
	}
	contentType := r.Header.Get("Content-Type")
	idx := strings.Index(contentType, ";")
	if idx != -1 {
		contentType = contentType[:idx]
	}
	var codec Codec
	if contentType == "" && len(s.codecs) == 1 {
		// If Content-Type is not set and only one codec has been registered,
		// then default to that codec.
		for _, c := range s.codecs {
			codec = c
		}
	} else if codec = s.codecs[strings.ToLower(contentType)]; codec == nil {
		WriteError(w, 415, "rpc: unrecognized Content-Type: "+contentType)
		return
	}

	pathMethod := LastPart(r.URL.Path)
	_, errGet := s.service.Get(pathMethod)
	if errGet != nil {
		WriteError(w, 404, errGet.Error())
		return
	}

	// Create a new codec request.
	codecReq := codec.NewRequest(r)

	if codecReq.Error() != nil {
		codecReq.WriteError(w, 400, codecReq.Error())
		return
	}

	// Get service method to be called.
	methodName, errMethod := codecReq.Method()
	if errMethod != nil {
		codecReq.WriteError(w, 400, errMethod)
		return
	}

	methodSpec, errGet := s.service.Get(methodName)
	if errGet != nil {
		codecReq.WriteError(w, 400, errGet)
		return
	}
	// Decode the args.
	args := reflect.New(methodSpec.argsType)
	if errRead := codecReq.ReadRequest(args.Interface()); errRead != nil {
		codecReq.WriteError(w, 400, errRead)
		return
	}
	// Call the service method.
	reply := reflect.New(methodSpec.replyType)
	errValue := methodSpec.method.Func.Call([]reflect.Value{
		s.service.rcvr,
		reflect.ValueOf(r),
		args,
		reply,
	})
	// Cast the result to error if needed.
	var errResult error
	errInter := errValue[0].Interface()
	if errInter != nil {
		errResult = errInter.(error)
	}

	// Encode the response.
	if errResult == nil {
		codecReq.WriteResponse(w, reply.Interface())
	} else {
		codecReq.WriteError(w, 400, errResult)
	}
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, msg)
}
