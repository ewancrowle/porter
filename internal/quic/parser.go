package quic

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type CryptoAssembler struct {
	buffer []byte
	offset uint64
}

func NewCryptoAssembler() *CryptoAssembler {
	return &CryptoAssembler{
		buffer: make([]byte, 0),
		offset: 0,
	}
}

func (ca *CryptoAssembler) HandleFrame(data []byte) ([]byte, error) {
	// A CRYPTO frame:
	// 0x06 (type)
	// Offset (variable length integer)
	// Length (variable length integer)
	// Data
	if len(data) < 1 || data[0] != 0x06 {
		return nil, nil // Not a CRYPTO frame
	}

	curr := 1
	offset, n, err := ReadVarInt(data[curr:])
	if err != nil {
		return nil, fmt.Errorf("invalid offset in CRYPTO frame: %v", err)
	}
	curr += n

	length, n, err := ReadVarInt(data[curr:])
	if err != nil {
		return nil, fmt.Errorf("invalid length in CRYPTO frame: %v", err)
	}
	curr += n

	if len(data) < curr+int(length) {
		return nil, errors.New("CRYPTO frame data too short")
	}

	frameData := data[curr : curr+int(length)]

	// Simple assembly: we expect frames to be in order for Initial
	// If they are not, this simplified assembler might fail.
	// But usually Initial CRYPTO frames are small and often fit in one packet.

	if offset == ca.offset {
		ca.buffer = append(ca.buffer, frameData...)
		ca.offset += length
	} else if offset < ca.offset {
		// Duplicate or overlap, ignore for simplicity if it doesn't extend
		end := offset + length
		if end > ca.offset {
			overlap := ca.offset - offset
			ca.buffer = append(ca.buffer, frameData[overlap:]...)
			ca.offset = end
		}
	} else {
		return nil, fmt.Errorf("out of order CRYPTO frame: expected %d, got %d", ca.offset, offset)
	}

	return ca.buffer, nil
}

func ReadVarInt(data []byte) (uint64, int, error) {
	if len(data) == 0 {
		return 0, 0, errors.New("data too short")
	}
	// The first 2 bits determine the length
	prefix := data[0] >> 6
	length := 1 << prefix

	if len(data) < length {
		return 0, 0, errors.New("data too short for varint")
	}

	var val uint64
	// Mask off the prefix bits from the first byte
	val = uint64(data[0] & 0x3f)

	for i := 1; i < length; i++ {
		val = (val << 8) | uint64(data[i])
	}
	return val, length, nil
}

func ExtractSNIFromClientHello(data []byte) (string, error) {
	// TLS ClientHello starts after the Handshake header
	// Handshake Type (1 byte) + Length (3 bytes)
	if len(data) < 4 {
		return "", errors.New("too short for TLS Handshake")
	}
	if data[0] != 0x01 { // ClientHello
		return "", errors.New("not a ClientHello")
	}

	curr := 4
	if len(data) < curr+2 {
		return "", errors.New("too short for Version")
	}
	// Skip Version (2 bytes)
	curr += 2

	if len(data) < curr+32 {
		return "", errors.New("too short for Random")
	}
	// Skip Random (32 bytes)
	curr += 32

	if len(data) < curr+1 {
		return "", errors.New("too short for Legacy Session ID")
	}
	sidLen := int(data[curr])
	curr += 1 + sidLen

	if len(data) < curr+2 {
		return "", errors.New("too short for Cipher Suites")
	}
	csLen := int(binary.BigEndian.Uint16(data[curr:]))
	curr += 2 + csLen

	if len(data) < curr+1 {
		return "", errors.New("too short for Compression Methods")
	}
	cmLen := int(data[curr])
	curr += 1 + cmLen

	if len(data) < curr+2 {
		return "", errors.New("no extensions")
	}
	extensionsLen := int(binary.BigEndian.Uint16(data[curr:]))
	curr += 2
	extensionsEnd := curr + extensionsLen

	if len(data) < extensionsEnd {
		return "", errors.New("extensions truncated")
	}

	for curr < extensionsEnd {
		if curr+4 > extensionsEnd {
			break
		}
		extType := binary.BigEndian.Uint16(data[curr:])
		extLen := int(binary.BigEndian.Uint16(data[curr+2:]))
		curr += 4

		if extType == 0 { // server_name
			if curr+extLen > extensionsEnd {
				return "", errors.New("SNI extension truncated")
			}
			sniData := data[curr : curr+extLen]
			if len(sniData) < 2 {
				return "", errors.New("invalid SNI extension data")
			}
			// SNI List Length (2 bytes)
			// SNI Type (1 byte) - 0 for host_name
			// SNI Name Length (2 bytes)
			// SNI Name
			sniListLen := int(binary.BigEndian.Uint16(sniData))
			if len(sniData) < 2+sniListLen {
				return "", errors.New("SNI list truncated")
			}

			subCurr := 2
			for subCurr < 2+sniListLen {
				if subCurr+3 > 2+sniListLen {
					break
				}
				nameType := sniData[subCurr]
				nameLen := int(binary.BigEndian.Uint16(sniData[subCurr+1:]))
				subCurr += 3
				if nameType == 0 {
					if subCurr+nameLen > 2+sniListLen {
						return "", errors.New("SNI name truncated")
					}
					return string(sniData[subCurr : subCurr+nameLen]), nil
				}
				subCurr += nameLen
			}
		}
		curr += extLen
	}

	return "", errors.New("SNI not found")
}
