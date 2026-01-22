package quic

import (
	"encoding/hex"
	"testing"
)

func TestDeriveInitialKeys(t *testing.T) {
	// Standard test vector from RFC 9001 Appendix A.1
	dcid, _ := hex.DecodeString("8394c8f03e515708")
	clientKeys, _ := deriveInitialKeys(dcid, false)

	if len(clientKeys.key) != 16 {
		t.Errorf("Expected 16 byte key, got %d", len(clientKeys.key))
	}
	if len(clientKeys.iv) != 12 {
		t.Errorf("Expected 12 byte IV, got %d", len(clientKeys.iv))
	}
	if len(clientKeys.hp) != 16 {
		t.Errorf("Expected 16 byte HP key, got %d", len(clientKeys.hp))
	}

	expectedClientKey := "1f369613dd76d5467730efcbe3b1a22d"
	if hex.EncodeToString(clientKeys.key) != expectedClientKey {
		t.Errorf("Client key mismatch. Got %x, want %s", clientKeys.key, expectedClientKey)
	}

	expectedClientIV := "fa044b2f42a3fd3b46fb255c"
	if hex.EncodeToString(clientKeys.iv) != expectedClientIV {
		t.Errorf("Client IV mismatch. Got %x, want %s", clientKeys.iv, expectedClientIV)
	}

	expectedClientHP := "9f50449e04a0e810283a1e9933adedd2"
	if hex.EncodeToString(clientKeys.hp) != expectedClientHP {
		t.Errorf("Client HP key mismatch. Got %x, want %s", clientKeys.hp, expectedClientHP)
	}
}

func TestParsePacketUnsupportedVersion(t *testing.T) {
	// Greased version or unsupported version
	data := []byte{0x80, 0x8d, 0xb3, 0x3e, 0x9b, 0x00}
	_, err := ParsePacket(data)
	if err == nil {
		t.Error("Expected error for unsupported version")
	}
	if err.Error() != "unsupported QUIC version" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestParsePacketVersionNegotiation(t *testing.T) {
	data := []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00}
	_, err := ParsePacket(data)
	if err == nil {
		t.Error("Expected error for version negotiation packet")
	}
	if err.Error() != "version negotiation packet" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestExtractSNIFromClientHello(t *testing.T) {
	// Mock TLS ClientHello with SNI
	clientHello := []byte{
		0x01,             // Handshake Type: ClientHello
		0x00, 0x00, 0x2b, // Length
		0x03, 0x03, // Version
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // Random
		0x00,                   // Session ID Length
		0x00, 0x02, 0x13, 0x01, // Cipher Suites
		0x01, 0x00, // Compression Methods
		0x00, 0x14, // Extensions Length
		0x00, 0x00, // Extension: server_name
		0x00, 0x10, // Extension Length
		0x00, 0x0e, // SNI List Length
		0x00,       // Type: host_name
		0x00, 0x0b, // Name Length
		'e', 'x', 'a', 'm', 'p', 'l', 'e', '.', 'c', 'o', 'm',
	}

	sni, err := ExtractSNIFromClientHello(clientHello)
	if err != nil {
		t.Fatalf("ExtractSNIFromClientHello failed: %v", err)
	}
	if sni != "example.com" {
		t.Errorf("Expected example.com, got %s", sni)
	}
}

func TestReadVarInt(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantVal uint64
		wantLen int
		wantErr bool
	}{
		{"1 byte", []byte{0x25}, 37, 1, false},
		{"2 bytes", []byte{0x7b, 0xbd}, 15293, 2, false},
		{"4 bytes", []byte{0x9d, 0x7f, 0x3e, 0x7d}, 494878333, 4, false},
		{"8 bytes", []byte{0xc2, 0x19, 0x7c, 0x5e, 0xff, 0x14, 0xe8, 0x8c}, 151288809941952652, 8, false},
		{"too short", []byte{0x40}, 0, 0, true},
		{"empty", []byte{}, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotLen, err := ReadVarInt(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadVarInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotVal != tt.wantVal {
				t.Errorf("ReadVarInt() gotVal = %v, want %v", gotVal, tt.wantVal)
			}
			if gotLen != tt.wantLen {
				t.Errorf("ReadVarInt() gotLen = %v, want %v", gotLen, tt.wantLen)
			}
		})
	}
}
