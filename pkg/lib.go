package covertdns

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	LOG_LEVEL                  = slog.LevelDebug
	MSG_BUFFER_CAPACITY        = 2048
	MaxTXTRecordSize           = 60
	SERVER_ANSWER_CHUNK_LENGTH = 10
	HandshakeRequestOpCode     = 2
	HandshakeResponseOpCode    = 3
)

// DNSServer struct is used to create a DNS server that listens on a specific port.
type DNSServer struct {
	// The UDP connection object straight from the net package.
	conn *net.UDPConn

	// The port on which the server is listening.
	port string

	// The maximum number of answers that can be sent in a single response.
	// This number must fit inside the client response buffer which by default is
	// 1024 bytes
	maxAnswersPerResponse int

	// Dumb name to respond to the client.
	// This offers a way to control the name that an e.g. IPS would parse
	// We believe that a popular name would raise less suspicion. This must not end with a dot
	// as the DNS specification requires the canonical name to be trailed with a dot which is added later
	dumbName string

	// Injected slogger
	slog *slog.Logger
}

// NewDNSServer creates a new DNSServer object and returns it.
// By the time the function returns, the server is already listening on the specified port.
func NewDNSServer(port string, _logger *slog.Logger) (*DNSServer, error) {
	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	var _lgr *slog.Logger
	if _logger == nil {
		_lgr = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: LOG_LEVEL}))
	} else {
		_lgr = _logger
	}
	return &DNSServer{
		conn:                  conn,
		port:                  port,
		maxAnswersPerResponse: SERVER_ANSWER_CHUNK_LENGTH,
		dumbName:              "youtube.com",
		slog:                  _lgr,
	}, nil
}

// Close closes the UDP connection of the server.
func (_srv *DNSServer) Close() error {
	return _srv.conn.Close()
}

// The _handShakeResponse function is a helper function that sends a handshake response to the client.
// The handshake response is identified by the OpCode that is equal to HandshakeResponseOpCode.
func (_srv *DNSServer) _handShakeResponse(_msg dnsmessage.Message, addr *net.UDPAddr) error {
	if _msg.Header.OpCode != HandshakeRequestOpCode {
		return fmt.Errorf("not a handshake request")
	}
	// Construct the handshake response message
	response := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:       1,
			Response: true,
			OpCode:   HandshakeResponseOpCode,
		},
		Questions: []dnsmessage.Question{},
		Answers:   []dnsmessage.Resource{},
	}

	// Send the handshake response message to the client
	packed, err := response.Pack()
	if err != nil {
		return err
	}
	// _srv.conn.SetReadDeadline((time.Now().Add(2 * time.Second)))
	_, err = _srv.conn.WriteToUDP(packed, addr)
	if err != nil {
		return err
	}
	return nil
}

// Handle is the main function of the DNSServer struct. It listens for incoming DNS queries and
// calls the provided handler function to handle the queries. The handler function must return
// a base64 encoded string which will be sent back as a response to the client. The handler function
// must have the following signature: func(string) (string, error) where the input is the DNS query
// and the output is the DNS response. They must bye encoded in base64 in order for the other side to
// decode them. In later versions of this package, the encoding and decoding will be done automatically.
func (_srv *DNSServer) Handle(_handler func([]byte) ([]byte, error)) {
	_msgs := make(map[string][]dnsmessage.Message)
	go func() {
		for {
			time.Sleep(1 * time.Second)
			for _addr, _msg := range _msgs {

				if len(_msg) == 0 || _msg[len(_msg)-1].Header.OpCode != 0 {
					continue
				}
				// Parse the first question of every message and concatenate them into a single base64 string
				_b64, err1 := _srv._parseFirstQuestionFromMultipleMsgs(_msg)
				if err1 != nil {
					_srv.slog.Error("Error parsing DNS query:", err1.Error(), "")
					delete(_msgs, _addr)
					break
				}
				// _decodedB64, err2 := base64.StdEncoding.DecodeString(strings.TrimSpace(_b64))
				// if err2 != nil {
				// 	continue
				// }
				// // Decrypt the base64 string
				// _bytesB64, err3 := _decryptAes([]byte(_decodedB64), _srv.aesKey)
				// if err3 != nil {
				// 	_srv.slog.Error("Could not decrypt request: ", err3.Error(), "")
				// 	continue
				// }

				// Call the handler function to handle the DNS question. B64 string goes in
				// and B64 string comes out.
				base64bytes, err := _handler([]byte(_b64))
				if err != nil { // Here we need to remove the message from the map and then move on to the next one
					_srv.slog.Error("Error handling DNS query:", err.Error(), "")
					delete(_msgs, _addr)
					break
				}

				// Encrypt the response
				// base64BytesEncrypted, err4 := _encryptAes([]byte(base64String), _srv.aesKey)
				// if err4 != nil {
				// 	_srv.slog.Error("Could not encrypt response: ", err4.Error(), "")
				// 	continue
				// }
				// base64StringEncrypted := base64.StdEncoding.EncodeToString(base64BytesEncrypted)
				// Pack the response into a DNS message and send it back to the client. At the moment the
				// rr type is always TXT so the second argument is not necessary
				_asnswerList, err := _srv._base64ToMultipleAnswersOfOneTxtRecord(string(base64bytes), dnsmessage.TypeTXT)
				if err != nil {
					_srv.slog.Error("Error constructing DNS response:", err.Error(), "")
					delete(_msgs, _addr)
					break
				}

				// Now split the response to chunks of maxAnswersPerResponse and send them to the client
				_answersLength := len(_asnswerList)
				_answersChunks := make([][]dnsmessage.Resource, 0)

				for i := 0; i <= _answersLength/_srv.maxAnswersPerResponse; i++ {
					// If the remaining answers are less than maxAnswersPerResponse, then add them all
					if len(_asnswerList) < _srv.maxAnswersPerResponse {
						_answersChunks = append(_answersChunks, _asnswerList)
						break
					}
					_answersChunks = append(_answersChunks, _asnswerList[:_srv.maxAnswersPerResponse])
					_asnswerList = _asnswerList[_srv.maxAnswersPerResponse:]
				}

				for i, _answerChunk := range _answersChunks {
					var opcode uint16
					// Again set OpCode to 0 if this is the last chunk
					if i == len(_answersChunks)-1 {
						opcode = 0
					} else {
						opcode = 1
					}
					response := dnsmessage.Message{
						Header: dnsmessage.Header{
							ID:       1,
							Response: true,
							OpCode:   dnsmessage.OpCode(opcode),
						},
						Questions: []dnsmessage.Question{},
						Answers:   _answerChunk,
					}

					// log.Printf("Sending response: %+v\n", response)
					packed, err := response.Pack()
					if err != nil {
						_srv.slog.Warn("Error packing DNS response:", err.Error(), "")
						break
					}

					_convAddr, err := AddrStrToUdpAddr(_addr)
					if err != nil {
						_srv.slog.Error("Could not parse client's address: ", _addr, "")
						break
					}

					_, err = _srv.conn.WriteToUDP(packed, _convAddr)
					if err != nil {
						_srv.slog.Warn("Error writing DNS response:", err.Error(), "")
						break
					}
				}
				// Clear the messages
				delete(_msgs, _addr)
				break
			}
		}
	}()
	for {
		// Read the incoming DNS message
		tempBuf := make([]byte, MSG_BUFFER_CAPACITY)
		for {
			// Read chunks
			n, addr, err1 := _srv.conn.ReadFromUDP(tempBuf)
			if err1 != nil {
				_srv.slog.Warn("Error reading from UDP connection:", err1.Error(), "")
				continue
			}
			_tmpMsg := dnsmessage.Message{}
			// Read the message
			err2 := _tmpMsg.Unpack(tempBuf[:n])
			if err2 != nil {
				tempBuf = make([]byte, MSG_BUFFER_CAPACITY)
				continue
			}
			addrStr := fmt.Sprintf("%s:%d", addr.IP.String(), addr.Port)

			if _, exists := _msgs[addrStr]; !exists {
				_msgs[addrStr] = []dnsmessage.Message{}
			}

			err3 := _srv._handShakeResponse(_tmpMsg, addr)
			if err3 != nil {
				if err3.Error() != "not a handshake request" {
					_srv.slog.Warn("Error sending handshake response:", err3.Error(), "")
					continue
				}
			} else {
				tempBuf = make([]byte, MSG_BUFFER_CAPACITY)
				continue
			}
			tempBuf = make([]byte, MSG_BUFFER_CAPACITY)
			// Check if the OpCode is 0 or 1
			// OpCode 0 means that this is the last question
			// OpCode 1 means that there are more questions to come
			if _tmpMsg.Header.OpCode == 0 {
				if len(_msgs[addrStr]) > 0 {
					if _msgs[addrStr][len(_msgs[addrStr])-1].ID == _tmpMsg.ID-1 {
						_msgs[addrStr] = append(_msgs[addrStr], _tmpMsg)
						tempBuf = make([]byte, MSG_BUFFER_CAPACITY)
						break
					}
				} else {
					_msgs[addrStr] = append(_msgs[addrStr], _tmpMsg)
					break
				}
			} else if _tmpMsg.Header.OpCode == 1 {
				if len(_msgs[addrStr]) > 0 {
					if _msgs[addrStr][len(_msgs[addrStr])-1].ID == _tmpMsg.ID-1 {
						_msgs[addrStr] = append(_msgs[addrStr], _tmpMsg)
					}
				} else {
					_msgs[addrStr] = append(_msgs[addrStr], _tmpMsg)
				}
			} else {
				tempBuf = make([]byte, MSG_BUFFER_CAPACITY)
				break
			}
		}
	}
}

// Is a helper function that parses the first question from a list of DNS messages sent from a client
// It concatenates the questions and returns the final base64 encoded string.
func (_srv *DNSServer) _parseFirstQuestionFromMultipleMsgs(_msgs []dnsmessage.Message) (string, error) {
	_finalBase64String := ""
	_chunked := ""

	if len(_msgs) == 0 {
		return "", fmt.Errorf("No questions found in the message")
	}
	for _, _msg := range _msgs {
		if len(_msg.Questions) == 0 {
			return "", fmt.Errorf("No questions found in the message")
		}

		switch _msg.Questions[0].Class {
		case dnsmessage.ClassINET:
			_chunked += RemoveLastCharFromByteList(_msg.Questions[0].Name.Data)
		case dnsmessage.ClassCSNET:
			_chunked += RemoveLastCharFromByteList(_msg.Questions[0].Name.Data)
			_finalBase64String += _chunked
			_chunked = ""
		case dnsmessage.ClassANY:
			_finalBase64String += RemoveLastCharFromByteList(_msg.Questions[0].Name.Data)
		default:
			slog.Info("Not supported class: %v\n", _msg.Questions[0].Class.String(), "")
		}

	}

	return _finalBase64String, nil
}

// Constructs the DNS response message from the base64 encoded string.
// The base64 string is split into chunks of 63 characters and each chunk is added as a separate answer with one TXT record.
func (_srv *DNSServer) _base64ToMultipleAnswersOfOneTxtRecord(_inb64String string, _mType dnsmessage.Type) ([]dnsmessage.Resource, error) {
	_base64String := _inb64String
	_answerList := []dnsmessage.Resource{}

	var _chunks int
	if len(_base64String)%MaxTXTRecordSize > 0 {
		_chunks = len(_base64String)/MaxTXTRecordSize + 1
	} else {
		_chunks = len(_base64String) / MaxTXTRecordSize
	}

	var _tmpString string
	_class := dnsmessage.ClassINET
	for j := 1; j <= _chunks; j++ {
		if _chunks == 1 {
			_tmpString = _base64String
			_class = dnsmessage.ClassANY
		} else {
			if j == _chunks {
				_tmpString = _base64String[(j-1)*MaxTXTRecordSize:]
				_class = dnsmessage.ClassCSNET
			} else {
				_tmpString = _base64String[(j-1)*MaxTXTRecordSize : j*MaxTXTRecordSize]
				_class = dnsmessage.ClassINET
			}
		}

		_answerList = append(_answerList, dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  dnsmessage.MustNewName(_srv.dumbName + "."),
				Type:  _mType,
				Class: _class,
			},
			Body: &dnsmessage.TXTResource{
				TXT: []string{_tmpString},
			},
		})
	}

	return _answerList, nil
}

type DNSClient struct {
	// The UDP connection object straight from the net package.
	conn *net.UDPConn
	// The address of the remote DNS server
	address string
	// The port of the remote DNS server.
	port string
	// Timeout waiting for server
	timeout time.Duration
	// Injected slogger
	slog *slog.Logger
	// Handshake tries
	handshakeTries int
	// Max delay between DNS requests
	maxMillisecondsDelayBetweenRequests int
}

type Ping struct {
	P int8 "json:\"p\""
}

func NewDNSClient(address, port string, _handshakeTries, _maxDelayBetweenRequests int, _logger *slog.Logger) (*DNSClient, error) {
	_addr, err := net.ResolveUDPAddr("udp", address+":"+port)
	if err != nil {
		return nil, err
	}
	_conn, err := net.DialUDP("udp", nil, _addr)
	if err != nil {
		return nil, err
	}
	var _lgr *slog.Logger
	if _logger == nil {
		_lgr = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: LOG_LEVEL}))
	} else {
		_lgr = _logger
	}

	return &DNSClient{
		conn:                                _conn,
		address:                             address,
		port:                                port,
		timeout:                             5 * time.Second,
		slog:                                _lgr,
		handshakeTries:                      _handshakeTries,
		maxMillisecondsDelayBetweenRequests: _maxDelayBetweenRequests,
	}, nil
}

func (_dnsc *DNSClient) Close() error {
	return _dnsc.conn.Close()
}

// Constructs the DNS question message from the base64 encoded string. It returns a slice of questions
// The base64 string is as always, split into chunks of 63 characters and each chunk is added as a separate question with one TXT record.
func (_dnsc *DNSClient) QuestionMessageAssembler(_inb64String string) ([]dnsmessage.Question, error) {
	_base64String := _inb64String
	_questionList := []dnsmessage.Question{}

	var _chunks int
	if len(_base64String)%MaxTXTRecordSize > 0 {
		_chunks = len(_base64String)/MaxTXTRecordSize + 1
	} else {
		_chunks = len(_base64String) / MaxTXTRecordSize
	}
	var _tmpString string
	_class := dnsmessage.ClassINET

	for j := 1; j <= _chunks; j++ {
		if _chunks == 1 {
			_tmpString = _base64String
			_class = dnsmessage.ClassANY
		} else {
			if j == _chunks {
				_tmpString = _base64String[(j-1)*MaxTXTRecordSize:]
				_class = dnsmessage.ClassCSNET
			} else {
				_tmpString = _base64String[(j-1)*MaxTXTRecordSize : j*MaxTXTRecordSize]
				_class = dnsmessage.ClassINET
			}
		}

		_questionList = append(_questionList, dnsmessage.Question{
			Name:  dnsmessage.MustNewName(_tmpString + "."),
			Type:  dnsmessage.TypeTXT,
			Class: _class,
		})
	}

	return _questionList, nil
}

// Parses the first TXT record from all the answers in the DNS response message. It concatenates all the TXT records
// and returns the final base64 encoded string. This is a better method than fitting all the TXT records in a slice in
// the first answer because it gives us the flexibility to break the messages in as many answers as we want.
func (_dnsc *DNSClient) _parseFirstTxtFromAllAnswers(_msg dnsmessage.Message) (string, error) {
	_finalBase64String := ""
	_chunked := ""

	if len(_msg.Answers) == 0 {
		return "", fmt.Errorf("No answers found in the message")
	}
	for _, answer := range _msg.Answers {
		_tt := answer.Body.(*dnsmessage.TXTResource).TXT[0]

		switch answer.Header.Class {
		case dnsmessage.ClassINET:
			_chunked += _tt
		case dnsmessage.ClassCSNET:
			_chunked += _tt
			_finalBase64String += _chunked
			_chunked = ""
		case dnsmessage.ClassANY:
			_finalBase64String += _tt
		default:
			slog.Info("Not supported class: %v\n", answer.Header.Class.String(), "")
		}
	}

	return _finalBase64String, nil
}

// A custom handshake-like udp request that sends DNS OpCode 2 (handshake request opCode)
// and waits for the server to respond with OpCode 3 (handshake response OpCode)
// If anything fails it returns an error.
func (_dnsc *DNSClient) _handShakeRequest() error {
	query := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:               1,
			OpCode:           HandshakeRequestOpCode,
			RecursionDesired: true,
			Response:         false,
		},
		Questions: []dnsmessage.Question{},
	}

	packedQuery, err := query.Pack()
	if err != nil {
		return fmt.Errorf("Error packing DNS custom handshake: %v", err)
	}
	_tries := 0
O:
	for {
		if _tries >= _dnsc.handshakeTries {
			return fmt.Errorf("Max handshake tries reached")
		}
		// This is a small workaround to wait for the buffer to be cleared
		_dnsc.conn.SetReadDeadline((time.Now().Add(_dnsc.timeout)))
		_, werr := _dnsc.conn.Write(packedQuery)
		if werr != nil {
			return fmt.Errorf("Handshake write error: %v", werr)
		}
		tmpBuffer := make([]byte, MSG_BUFFER_CAPACITY)

		for {
			_, _, err := _dnsc.conn.ReadFromUDP(tmpBuffer)
			if err != nil {
				if strings.Contains(err.Error(), "i/o timeout") {
					_tries++
					break
				} else {
					_dnsc.slog.Debug("Handshake generic error: ", err.Error(), "")
					break O
				}
			}
			var parser dnsmessage.Parser
			_header, err := parser.Start(tmpBuffer)
			if err != nil {
				return err
			}

			if _header.OpCode == HandshakeResponseOpCode {
				return nil
			}
		}
		_dnsc.Sleep()
	}
	return fmt.Errorf("Handshake failed")
}

func (_dnsc *DNSClient) Sleep() int {
	_minimumDelay := 200
	_newSleepTime := rand.IntN(_dnsc.maxMillisecondsDelayBetweenRequests)
	time.Sleep(time.Duration(_minimumDelay+_newSleepTime) * time.Millisecond)
	return _minimumDelay + _newSleepTime
}

// Sends the base64 encoded string to the remote DNS server and returns the base64 encoded response.
func (_dnsc *DNSClient) Send(base64String []byte) ([]byte, error) {
	// encBase64Bytes, err1 := _encryptAes(base64String, _dnsc.aesKey)
	// if err1 != nil {
	// 	return nil, err1
	// }
	// encBase64String := base64.StdEncoding.EncodeToString(encBase64Bytes)
	questions, err2 := _dnsc.QuestionMessageAssembler(string(base64String))
	if err2 != nil {
		return nil, fmt.Errorf("Error assembling DNS questions: %v", err2)
	}

	base64Resp, err3 := _dnsc.SendQuery(questions)
	if err3 != nil {
		return nil, err3
	}
	return base64Resp, nil
}

// Sends the DNS query to the remote DNS server and returns the base64 encoded response.
func (_dnsc *DNSClient) SendQuery(questions []dnsmessage.Question) ([]byte, error) {
	err1 := _dnsc._handShakeRequest()
	// Remove deadline
	_dnsc.conn.SetDeadline(time.Now().Add(5000 * time.Second))
	if err1 != nil {
		return nil, fmt.Errorf("Handshake failed: %v", err1)
	}
	// After receiving the questions it breaks into separate DNS queries as most DNS implementations
	// do not support multiple questions in a single query.
	for i, q := range questions {
		query := dnsmessage.Message{
			Header: dnsmessage.Header{
				ID:               uint16(i),
				RecursionDesired: true,
				Response:         false,
			},
			Questions: []dnsmessage.Question{q},
		}
		// Set OpCode to 0 if this is the last question, otherwise set it to 1
		if i == len(questions)-1 {
			query.Header.OpCode = 0
		} else {
			query.Header.OpCode = 1
		}
		packedQuery, err2 := query.Pack()
		if err2 != nil {
			return nil, err2
		}
		_ = _dnsc.Sleep()
		_, err3 := _dnsc.conn.Write(packedQuery)
		if err3 != nil {
			_dnsc.slog.Debug("Error writing DNS query:", err3.Error(), "")
			continue
		}
	}

	var response dnsmessage.Message

	_answersList := []dnsmessage.Resource{}
	// Reads responses into chunks from the server until OpCode is 0
	tmpBuffer := make([]byte, MSG_BUFFER_CAPACITY)
	_responseTimeoutCounter := 0
	for {
		_, _, err4 := _dnsc.conn.ReadFromUDP(tmpBuffer)
		if err4 != nil {
			if strings.Contains(err4.Error(), "i/o timeout") {
				_responseTimeoutCounter++
				if _responseTimeoutCounter > 5 {
					return nil, fmt.Errorf("Timeout reading from server")
				}
			}
			continue
		}
		_responseTimeoutCounter = 0
		var parser dnsmessage.Parser
		_header, err5 := parser.Start(tmpBuffer)
		if err5 != nil {
			continue
		}

		parser.SkipAllQuestions()

		for {
			_res, err6 := parser.Answer()
			if err6 != nil {
				break
			}
			_answersList = append(_answersList, _res)
		}

		if _header.OpCode == 0 {
			break
		}
	}

	response.Answers = _answersList
	base64String, er7 := _dnsc._parseFirstTxtFromAllAnswers(response)
	if er7 != nil {
		return nil, er7
	}
	return []byte(base64String), nil
}
