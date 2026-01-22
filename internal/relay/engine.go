package relay

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/ewancrowle/porter/internal/config"
	"github.com/ewancrowle/porter/internal/quic"
	"github.com/ewancrowle/porter/internal/strategy"
)

type Relay struct {
	listenAddr *net.UDPAddr
	conn       *net.UDPConn
	manager    *strategy.StrategyManager
	cfg        *config.Config

	sessions sync.Map
}

type session struct {
	targetAddr *net.UDPAddr
	lastSeen   time.Time
	mu         sync.RWMutex
	srcAddr    *net.UDPAddr
}

func NewRelay(cfg *config.Config, manager *strategy.StrategyManager) (*Relay, error) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", cfg.UDP.Port))
	if err != nil {
		return nil, err
	}

	return &Relay{
		listenAddr: addr,
		manager:    manager,
		cfg:        cfg,
	}, nil
}

func (r *Relay) Start(ctx context.Context) error {
	conn, err := net.ListenUDP("udp", r.listenAddr)
	if err != nil {
		return err
	}
	r.conn = conn
	defer r.conn.Close()

	log.Printf("UDP Relay listening on %s", r.listenAddr.String())

	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, srcAddr, err := r.conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("Error reading from UDP: %v", err)
				continue
			}

			data := make([]byte, n)
			copy(data, buf[:n])

			go r.processUDPDatagram(srcAddr, data)
		}
	}
}

func (r *Relay) processUDPDatagram(srcAddr *net.UDPAddr, data []byte) {
	curr := 0
	for curr < len(data) {
		header, err := quic.ParsePacket(data[curr:])
		if err != nil {
			if r.cfg.UDP.LogRequests && curr == 0 {
				log.Printf("Relay: %s -> unknown (parse error: %v)", srcAddr, err)
			}
			return
		}

		packetData := data[curr : curr+header.FullLength]
		r.handlePacket(srcAddr, packetData, header)

		curr += header.FullLength
		if !header.IsLongHeader {
			// Short header packets are not coalesced with other packets in the same way,
			// or at least they are usually the last ones.
			// RFC 9000 says: "A sender MUST NOT coalesce multiple QUIC packets
			// into a single UDP datagram unless it is certain that the receiver
			// will be able to process them... only long header packets can be coalesced."
			// Actually, short header packets can be preceded by long header packets.
			// But once we see a short header, it's typically the end or we can't easily find the next one.
			break
		}
	}
}

func (r *Relay) handlePacket(srcAddr *net.UDPAddr, data []byte, header *quic.ParsedHeader) {
	dcid := string(header.DCID)
	srcStr := srcAddr.String()

	if val, ok := r.sessions.Load(dcid); ok {
		sess := val.(*session)
		sess.mu.Lock()
		if sess.srcAddr.String() != srcStr {
			if r.cfg.UDP.LogRequests {
				log.Printf("Relay: %s -> %s (migrated from %s, DCID: %x)", srcStr, sess.targetAddr, sess.srcAddr, header.DCID)
			}
			sess.srcAddr = srcAddr
		}
		targetAddr := sess.targetAddr
		sess.lastSeen = time.Now()
		sess.mu.Unlock()

		if r.cfg.UDP.LogRequests {
			log.Printf("Relay: %s -> %s (session, DCID: %x)", srcStr, targetAddr, header.DCID)
		}
		r.forward(targetAddr, data)
		return
	}

	if !header.IsLongHeader || header.Type != 0x00 {
		if r.cfg.UDP.LogRequests {
			log.Printf("Relay: %s -> unknown (no session and not an Initial packet, DCID: %x)", srcStr, header.DCID)
		}
		return
	}

	sni, err := quic.ExtractSNI(data)
	if err != nil {
		if r.cfg.UDP.LogRequests {
			log.Printf("Relay: %s -> unknown (failed to extract SNI: %v, DCID: %x)", srcStr, err, header.DCID)
		}
		return
	}

	target, err := r.resolveTarget(sni)
	if err != nil {
		if r.cfg.UDP.LogRequests {
			log.Printf("Relay: %s -> unknown (SNI: %s, error: %v, DCID: %x)", srcStr, sni, err, header.DCID)
		}
		log.Printf("Failed to resolve target for SNI %s: %v", sni, err)
		return
	}

	targetAddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		log.Printf("Invalid target address %s: %v", target, err)
		return
	}

	if r.cfg.UDP.LogRequests {
		log.Printf("Relay: %s -> %s (new session, SNI: %s, DCID: %x)", srcStr, target, sni, header.DCID)
	} else {
		log.Printf("New session: %s -> %s (SNI: %s, DCID: %x)", srcStr, target, sni, header.DCID)
	}

	newSess := &session{
		targetAddr: targetAddr,
		lastSeen:   time.Now(),
		srcAddr:    srcAddr,
	}
	r.sessions.Store(dcid, newSess)

	r.forward(targetAddr, data)
}

func (r *Relay) resolveTarget(sni string) (string, error) {
	if s := r.manager.Get(strategy.StrategySimple); s != nil {
		if target, err := s.Resolve(context.Background(), sni); err == nil {
			return target, nil
		}
	}

	if s := r.manager.Get(strategy.StrategyAgones); s != nil {
		if target, err := s.Resolve(context.Background(), sni); err == nil {
			return target, nil
		}
	}

	return "", fmt.Errorf("no route for SNI %s", sni)
}

func (r *Relay) forward(targetAddr *net.UDPAddr, data []byte) {
	outConn, err := net.DialUDP("udp", nil, targetAddr)
	if err != nil {
		log.Printf("Error dialing target %v: %v", targetAddr, err)
		return
	}
	defer outConn.Close()

	_, err = outConn.Write(data)
	if err != nil {
		log.Printf("Error writing to target %v: %v", targetAddr, err)
	}
}
