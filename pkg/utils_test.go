package covertdns

import (
	"encoding/base64"
	"net"
	"reflect"
	"testing"
)

type testMsg struct {
	ID      int      `json:"id"`
	Domain  string   `json:"domain"`
	Payload []byte   `json:"payload"`
	Tags    []string `json:"tags"`
}

// ---------- Encode64 / Decode64 ----------

func TestEncode64_ProducesBase64OfJSON(t *testing.T) {
	got, err := Encode64(map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("Encode64 returned error: %v", err)
	}
	want := base64.StdEncoding.EncodeToString([]byte(`{"a":1}`))
	if string(got) != want {
		t.Errorf("Encode64 = %q, want %q", got, want)
	}
}

func TestEncode64_UnmarshalableTypeReturnsError(t *testing.T) {
	tests := map[string]any{
		"channel":  make(chan int),
		"function": func() {},
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Encode64(payload)
			if err == nil {
				t.Fatalf("expected error, got nil (result %q)", got)
			}
			if got != nil {
				t.Errorf("expected nil result on error, got %q", got)
			}
		})
	}
}

func TestEncodeDecode64_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   testMsg
	}{
		{"zero value", testMsg{}},
		{"full struct", testMsg{ID: 42, Domain: "example.com", Payload: []byte{0, 1, 2, 255}, Tags: []string{"a", "b"}}},
		{"unicode", testMsg{Domain: "ünïcødé.🦀"}},
		{"empty slices", testMsg{Payload: []byte{}, Tags: []string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := Encode64(tt.in)
			if err != nil {
				t.Fatalf("Encode64: %v", err)
			}
			dec, err := Decode64[testMsg](enc)
			if err != nil {
				t.Fatalf("Decode64: %v", err)
			}
			if dec == nil {
				t.Fatal("Decode64 returned nil pointer without error")
			}
			if !reflect.DeepEqual(*dec, tt.in) {
				t.Errorf("round trip mismatch:\n got  %+v\n want %+v", *dec, tt.in)
			}
		})
	}
}

func TestEncodeDecode64_RoundTripPrimitives(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		enc, _ := Encode64("hello")
		dec, err := Decode64[string](enc)
		if err != nil || *dec != "hello" {
			t.Errorf("got (%v, %v), want (hello, nil)", dec, err)
		}
	})
	t.Run("int", func(t *testing.T) {
		enc, _ := Encode64(1234)
		dec, err := Decode64[int](enc)
		if err != nil || *dec != 1234 {
			t.Errorf("got (%v, %v), want (1234, nil)", dec, err)
		}
	})
	t.Run("slice", func(t *testing.T) {
		in := []string{"x", "y"}
		enc, _ := Encode64(in)
		dec, err := Decode64[[]string](enc)
		if err != nil || !reflect.DeepEqual(*dec, in) {
			t.Errorf("got (%v, %v), want (%v, nil)", dec, err, in)
		}
	})
}

func TestDecode64_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"invalid base64 characters", []byte("!!!not-base64!!!")},
		{"bad padding", []byte("abc")},
		{"valid base64 but not JSON", []byte(base64.StdEncoding.EncodeToString([]byte("not json")))},
		{"JSON of wrong type", []byte(base64.StdEncoding.EncodeToString([]byte(`"a string"`)))},
		{"empty input", []byte{}},
		{"nil input", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode64[testMsg](tt.input)
			if err == nil {
				t.Fatalf("expected error, got nil (result %+v)", got)
			}
			if got != nil {
				t.Errorf("expected nil result on error, got %+v", got)
			}
		})
	}
}

// ---------- AddrStrToUdpAddr ----------

func TestAddrStrToUdpAddr_Valid(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		wantIP   net.IP
		wantPort int
	}{
		{"ip only defaults to port 80", "127.0.0.1", net.ParseIP("127.0.0.1"), 80},
		{"ip and port", "127.0.0.1:12341", net.ParseIP("127.0.0.1"), 12341},
		{"dns port", "8.8.8.8:53", net.ParseIP("8.8.8.8"), 53},
		{"port zero", "10.0.0.1:0", net.ParseIP("10.0.0.1"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddrStrToUdpAddr(tt.addr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.IP.Equal(tt.wantIP) {
				t.Errorf("IP = %v, want %v", got.IP, tt.wantIP)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tt.wantPort)
			}
		})
	}
}

func TestAddrStrToUdpAddr_Errors(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{"non-numeric port", "127.0.0.1:abc"},
		{"empty port", "127.0.0.1:"},
		{"too many colons", "1.2.3.4:53:99"},
		// strings.Split on ":" can't handle IPv6, so these are rejected.
		{"ipv6 loopback", "::1"},
		{"bracketed ipv6 with port", "[::1]:53"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddrStrToUdpAddr(tt.addr)
			if err == nil {
				t.Fatalf("expected error for %q, got %+v", tt.addr, got)
			}
			if got != nil {
				t.Errorf("expected nil result on error, got %+v", got)
			}
		})
	}
}

// Documents current behaviour: an unparsable host is NOT an error,
// it just yields a nil IP. Change this test if you add validation.
func TestAddrStrToUdpAddr_InvalidHostGivesNilIP(t *testing.T) {
	for _, addr := range []string{"localhost:53", "not-an-ip", ""} {
		t.Run(addr, func(t *testing.T) {
			got, err := AddrStrToUdpAddr(addr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.IP != nil {
				t.Errorf("IP = %v, want nil", got.IP)
			}
		})
	}
}

// ---------- RemoveLastCharFromByteList ----------

func toByteArray(s string) [255]byte {
	var b [255]byte
	copy(b[:], s)
	return b
}

func TestRemoveLastCharFromByteList(t *testing.T) {
	full := make([]byte, 255)
	for i := range full {
		full[i] = 'a'
	}

	tests := []struct {
		name string
		in   [255]byte
		want string
	}{
		{"strips trailing dot of domain", toByteArray("example.com."), "example.com"},
		{"single char", toByteArray("x"), ""},
		{"all zero bytes", [255]byte{}, ""},
		{"multi-byte rune removed whole", toByteArray("café"), "caf"},
		{"emoji removed whole", toByteArray("hi🦀"), "hi"},
		{"only trailing nulls trimmed", toByteArray("ab\x00cd"), "ab\x00c"},
		{"full array, no nulls", toByteArray(string(full)), string(full[:254])},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemoveLastCharFromByteList(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
