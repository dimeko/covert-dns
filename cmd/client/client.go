package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	libcovertdns "github.com/dimeko/covert-dns/pkg"
)

var (
	AES_KEY = []byte("0123456789abcdef")
	port    = "12341" // default port
	server  = "127.0.0.1"
)

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

func Send(client *libcovertdns.DNSClient, payload RequestPayload, wg *sync.WaitGroup) {
	base64bytes, err := libcovertdns.Encode64(payload)
	if err != nil {
		log.Println("Error encoding BigStruct instance:", err)
		wg.Done()
		return
	}

	_base64Resp, err := client.Send(base64bytes)
	if err != nil {
		log.Println("Error sending DNS query:", err)
		wg.Done()

		return
	}
	_resp, err := libcovertdns.Decode64[ResponsePayload](_base64Resp)
	if err != nil {
		log.Println("Error decoding DNS response:", err)
		wg.Done()

		return
	}
	log.Printf("Response %+v\n", _resp)
	wg.Done()
}

func main() {

	if len(os.Args) > 3 {
		for i := 0; i < len(os.Args); i++ {
			if os.Args[i] == "-p" && i+1 < len(os.Args) {
				port = os.Args[i+1]
				continue
			}
			if os.Args[i] == "-s" && i+1 < len(os.Args) {
				server = os.Args[i+1]
			}
		}
	} else {
		log.Println("Usage: go run main.go -p <port> -s <server>")
		return
	}

	fmt.Println("Connecting to server:", server, "on port:", port)
	_client, err := libcovertdns.NewDNSClient(server, port, 1, 500, nil)
	if err != nil {
		log.Println("Error creating DNS client:", err)
		return
	}

	defer _client.Close()
	var wg sync.WaitGroup

	counter := 0
	for counter < 3 {
		wg.Add(1)
		bigStructInstance := RequestPayload{
			Field1:  "Value1",
			Field2:  "Value2Value2Value2Value2Value2Value2Value2Value2Value2Value2Value2Value2",
			Field3:  "Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3Value3",
			Field4:  "Value4",
			Field5:  "Value5",
			Field6:  []string{"Value6_1", "Value6_2", "Value6_3", "Value6_1", "Value6_2", "Value6_3", "Value6_1", "Value6_2", "Value6_3", "Value6_1", "Value6_2", "Value6_3", "Value6_1", "Value6_2", "Value6_3", "Value6_1", "Value6_2", "Value6_3", "Value6_1", "Value6_2", "Value6_3"},
			Field7:  []string{"Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3", "Value7_1", "Value7_2", "Value7_3"},
			Field8:  []string{"Value8_1", "Value8_2", "Value8_3"},
			Field9:  map[string]string{"Key1": "Value9_1", "Key2": "Value9_2"},
			Field10: int64(1),
			Field11: []SliceItem{
				{Item: "Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1Item1"},
				{Item: "Item2"},
				{Item: "Item3"},
			},
		}
		Send(_client, bigStructInstance, &wg)
		counter++
	}
	wg.Wait()
}
