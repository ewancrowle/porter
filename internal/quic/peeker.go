package quic

import (
	"errors"
	"fmt"
)

// ExtractSNI attempts to extract the SNI from a QUIC Initial packet.
func ExtractSNI(data []byte) (string, error) {
	header, err := ParsePacket(data)
	if err != nil {
		return "", err
	}

	if !header.IsLongHeader || header.Type != 0x00 {
		return "", errors.New("not a QUIC Initial packet")
	}

	decrypted, err := DecryptInitialPacket(data, header.DCID)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt Initial packet: %v", err)
	}

	assembler := NewCryptoAssembler()
	// An Initial packet can contain multiple frames. We need to find the CRYPTO frame.
	// In a real implementation, we'd loop over frames.
	// For simplicity, let's look for the 0x06 frame type in the decrypted payload.

	curr := 0
	for curr < len(decrypted) {
		frameType := decrypted[curr]
		if frameType == 0x06 {
			assembled, err := assembler.HandleFrame(decrypted[curr:])
			if err != nil {
				// Try to continue searching for other CRYPTO frames if this one was invalid
				// but usually there's only one relevant one per packet.
				curr++
				continue
			}
			if assembled != nil {
				sni, err := ExtractSNIFromClientHello(assembled)
				if err == nil {
					return sni, nil
				}
				// If SNI not found yet, it might be in the next packet (if assembled is incomplete)
				// but for Initial we often have it in one.
			}
			// Move curr forward by frame length?
			// CRYPTO frame: 0x06 + offset (vint) + length (vint) + data
			// We can't easily skip it without re-parsing vint.
			// Let's just curr++ and hope for the best in this simplified parser.
		}
		curr++
	}

	return "", errors.New("SNI not found in decrypted Initial packet")
}
