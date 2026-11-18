package covertdns

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode/utf8"
)

func Encode64[T any](payload T) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.AppendEncode(nil, raw), nil
}

func Decode64[T any](payload []byte) (*T, error) {
	raw, err := base64.StdEncoding.AppendDecode(nil, payload)
	if err != nil {
		return nil, err
	}
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func AddrStrToUdpAddr(addr string) (*net.UDPAddr, error) {
	parts := strings.Split(addr, ":")
	fmt.Println("parts: ", parts)
	switch len(parts) {
	case 1:
		return &net.UDPAddr{
			Port: 80,
			IP:   net.ParseIP(parts[0]),
		}, nil
	case 2:
		num, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("could not parse port of address: %s\n", addr)
		}
		return &net.UDPAddr{
			Port: num,
			IP:   net.ParseIP(parts[0]),
		}, nil
	default:
		return nil, fmt.Errorf("could not parse address: %s\n", addr)
	}
}

func RemoveLastCharFromByteList(byteList [255]byte) string {
	s := string(bytes.TrimRight(byteList[:], "\x00")) // or: b[:bytes.IndexByte(b[:], 0)] for C-style strings
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}
