package quic

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

var quicV1Salt = []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a}

type initialKeys struct {
	key    []byte
	iv     []byte
	hp     []byte
	header cipher.Block
}

func deriveInitialKeys(destConnID []byte, isServer bool) (*initialKeys, *initialKeys) {
	initialSecret := hkdf.Extract(sha256.New, destConnID, quicV1Salt)

	clientSecret := deriveSecret(initialSecret, "client in", 32)
	serverSecret := deriveSecret(initialSecret, "server in", 32)

	clientKeys := setupKeys(clientSecret)
	serverKeys := setupKeys(serverSecret)

	return clientKeys, serverKeys
}

func deriveSecret(secret []byte, label string, length int) []byte {
	fullLabel := "tls13 " + label
	info := make([]byte, 2+1+len(fullLabel)+1)
	binary.BigEndian.PutUint16(info[0:2], uint16(length))
	info[2] = uint8(len(fullLabel))
	copy(info[3:], fullLabel)
	info[3+len(fullLabel)] = 0

	out := make([]byte, length)
	k := hkdf.Expand(sha256.New, secret, info)
	_, _ = io.ReadFull(k, out)
	return out
}

func setupKeys(secret []byte) *initialKeys {
	key := deriveSecret(secret, "quic key", 16)
	iv := deriveSecret(secret, "quic iv", 12)
	hpSecret := deriveSecret(secret, "quic hp", 16)

	block, _ := aes.NewCipher(hpSecret)

	return &initialKeys{
		key:    key,
		iv:     iv,
		hp:     hpSecret,
		header: block,
	}
}

type ParsedHeader struct {
	IsLongHeader bool
	Type         byte
	Version      uint32
	DCID         []byte
	SCID         []byte
	PacketNumber int64
	Payload      []byte
	RawHeader    []byte
	FullLength   int // Full length of the packet including header and payload
}

func ParsePacket(data []byte) (*ParsedHeader, error) {
	if len(data) < 1 {
		return nil, errors.New("packet too short")
	}

	header := &ParsedHeader{}
	firstByte := data[0]
	header.IsLongHeader = (firstByte & 0x80) != 0

	if header.IsLongHeader {
		if len(data) < 5 {
			return nil, errors.New("long header too short")
		}
		header.Version = binary.BigEndian.Uint32(data[1:5])
		header.Type = (firstByte & 0x30) >> 4

		// Strictly support Version 1
		if header.Version != 0x00000001 {
			if header.Version == 0x00000000 {
				return header, errors.New("version negotiation packet")
			}
			return header, errors.New("unsupported QUIC version")
		}

		curr := 5
		dcidLen := int(data[curr])
		curr++
		if len(data) < curr+dcidLen {
			return nil, errors.New("insufficient data for DCID")
		}
		header.DCID = data[curr : curr+dcidLen]
		curr += dcidLen

		scidLen := int(data[curr])
		curr++
		if len(data) < curr+scidLen {
			return nil, errors.New("insufficient data for SCID")
		}
		header.SCID = data[curr : curr+scidLen]
		curr += scidLen

		if header.Type == 0x00 { // Initial
			tokenLen, n, err := ReadVarInt(data[curr:])
			if err != nil {
				return nil, fmt.Errorf("invalid token length: %v", err)
			}
			curr += n
			if len(data) < curr+int(tokenLen) {
				return nil, errors.New("insufficient data for token")
			}
			curr += int(tokenLen)

			payloadLen, n, err := ReadVarInt(data[curr:])
			if err != nil {
				return nil, fmt.Errorf("invalid payload length: %v", err)
			}
			curr += n
			header.RawHeader = data[:curr]

			// Handle coalesced packets: the payload is only as long as payloadLen
			// The payload includes the Packet Number and the Frames.
			if len(data) < curr+int(payloadLen) {
				return nil, errors.New("insufficient data for payload")
			}
			header.Payload = data[curr : curr+int(payloadLen)]
			header.FullLength = curr + int(payloadLen)
		} else if header.Type == 0x01 || header.Type == 0x02 || header.Type == 0x03 {
			// Handshake, Retry, or 0-RTT also have a length field in many versions
			// but for now let's at least try to read it if it's there.
			// RFC 9000: Handshake and 0-RTT also have Length.
			payloadLen, n, err := ReadVarInt(data[curr:])
			if err == nil {
				curr += n
				header.RawHeader = data[:curr]
				if len(data) >= curr+int(payloadLen) {
					header.Payload = data[curr : curr+int(payloadLen)]
					header.FullLength = curr + int(payloadLen)
				} else {
					header.Payload = data[curr:]
					header.FullLength = len(data)
				}
			} else {
				header.RawHeader = data[:curr]
				header.Payload = data[curr:]
				header.FullLength = len(data)
			}
		} else {
			header.RawHeader = data[:curr]
			header.Payload = data[curr:]
			header.FullLength = len(data)
		}
	} else {
		// Short Header
		// We don't know the DCID length here, but usually it's fixed or negotiated.
		// For the sake of routing, we might need more context.
		// In a relay, we might assume a certain DCID length or have it from the session.
		// However, the issue says "Extract the DCID from the incoming packet header".
		// Short headers don't have a DCID length field.
		// Standard QUIC uses DCID that was negotiated.
		// Let's assume we can't fully parse short header without knowing DCID length.
		header.DCID = data[1 : 1+8] // HEURISTIC: Many implementations use 8 bytes
		header.FullLength = len(data)
	}

	return header, nil
}

func DecryptInitialPacket(data []byte, dcid []byte) ([]byte, error) {
	header, err := ParsePacket(data)
	if err != nil {
		return nil, err
	}
	// Redundant check because ParsePacket already enforces this, but good for safety
	if header.Version != 0x00000001 {
		return nil, errors.New("unsupported QUIC version")
	}
	if !header.IsLongHeader || header.Type != 0x00 {
		return nil, errors.New("not an initial packet")
	}

	clientKeys, _ := deriveInitialKeys(dcid, false)

	// Remove Header Protection
	// First byte (protected bits) and Packet Number are protected.
	// The PN offset is the end of the RawHeader (which includes everything up to but not including the PN)
	pnOffset := len(header.RawHeader)

	// Sample is taken from the payload. According to RFC 9001, for Initial packets,
	// the sample starts 4 bytes after the start of the Packet Number field.
	sampleOffset := pnOffset + 4
	if len(data) < sampleOffset+16 {
		return nil, errors.New("packet too short for sample")
	}
	sample := data[sampleOffset : sampleOffset+16]

	mask := make([]byte, 16)
	clientKeys.header.Encrypt(mask, sample)

	// Unmask first byte and Packet Number BEFORE reading values
	unprotectedFirstByte := data[0] ^ (mask[0] & 0x0f)
	pnLen := int((unprotectedFirstByte & 0x03) + 1)

	pnBytes := make([]byte, pnLen)
	for i := 0; i < pnLen; i++ {
		pnBytes[i] = data[pnOffset+i] ^ mask[i+1]
	}

	var packetNumber int64
	for _, b := range pnBytes {
		packetNumber = (packetNumber << 8) | int64(b)
	}

	// Construct AAD using the unprotected header
	aad := make([]byte, pnOffset+pnLen)
	copy(aad, data[:pnOffset])
	aad[0] = unprotectedFirstByte
	for i := 0; i < pnLen; i++ {
		aad[pnOffset+i] = pnBytes[i]
	}

	// Now we can decrypt the payload
	// The encrypted payload starts after the packet number
	realPayload := data[pnOffset+pnLen : header.FullLength]

	block, err := aes.NewCipher(clientKeys.key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], uint64(packetNumber))
	for i := 0; i < 12; i++ {
		nonce[i] ^= clientKeys.iv[i]
	}

	decrypted, err := aesgcm.Open(nil, nonce, realPayload, aad)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %v", err)
	}

	return decrypted, nil
}
