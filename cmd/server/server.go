package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	libcovertdns "github.com/dimeko/covert-dns/pkg"
)

var AES_KEY = []byte("0123456789abcdef")

type SliceItem struct {
	Item string `json:"item"`
}

type RequestPayload struct {
	Field1  string            `json:"field1"`
	Field2  string            `json:"field2"`
	Field3  string            `json:"field3"`
	Field4  string            `json:"field4"`
	Field5  string            `json:"field5"`
	Field6  []string          `json:"field6"`
	Field7  []string          `json:"field7"`
	Field8  []string          `json:"field8"`
	Field9  map[string]string `json:"field9"`
	Field10 int64             `json:"field10"`
	Field11 []SliceItem       `json:"field11"`
}

type ResponsePayload struct {
	Field6  []string
	Field9  map[string]string
	Field10 int64
}

func main() {
	port := "12341" // default port
	if len(os.Args) > 1 {
		for i := 0; i < len(os.Args); i++ {
			if os.Args[i] == "-p" && i+1 < len(os.Args) {
				port = os.Args[i+1]
				break
			}
		}
	}
	_srv, err := libcovertdns.NewDNSServer(port, nil)
	if err != nil {
		log.Println("Error creating DNS server:", err)
		return
	}
	fmt.Println("Listening on port:", port)

	defer _srv.Close()
	go _srv.Handle(
		func(payload []byte) ([]byte, error) {
			// log.Printf("Received payload: %s\n", payload)
			_req, err := libcovertdns.Decode64[RequestPayload](payload)
			if err != nil {
				return nil, fmt.Errorf("Error decoding payload: %s", err.Error())
			}
			_resp := ResponsePayload{
				Field6:  _req.Field6,
				Field9:  _req.Field9,
				Field10: _req.Field10,
			}

			log.Printf("Response payload: %+v\n", _resp)
			return libcovertdns.Encode64(_resp)
		},
	)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	log.Println("Shutting down server gracefully in 1 second.")
	time.Sleep(time.Second)
	defer cancel()
	os.Exit(0)
}
