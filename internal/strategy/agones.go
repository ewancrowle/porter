package strategy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"

	pb "agones.dev/agones/pkg/allocation/go"
	pkgerrors "github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type AgonesStrategy struct {
	mu     sync.RWMutex
	fleets map[string]string // FQDN -> Fleet Name

	enabled bool
	host    string
	cert    string
	key     string
	ca      string

	client pb.AllocationServiceClient
	conn   *grpc.ClientConn
}

func NewAgonesStrategy() *AgonesStrategy {
	return &AgonesStrategy{
		fleets: make(map[string]string),
	}
}

func (s *AgonesStrategy) Setup(enabled bool, host, cert, key, ca string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.enabled = enabled
	s.host = host
	s.cert = cert
	s.key = key
	s.ca = ca

	if !s.enabled {
		return nil
	}

	if s.conn != nil {
		s.conn.Close()
	}

	certBytes, err := os.ReadFile(s.cert)
	if err != nil {
		return fmt.Errorf("failed to read cert file: %w", err)
	}
	keyBytes, err := os.ReadFile(s.key)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}
	caBytes, err := os.ReadFile(s.ca)
	if err != nil {
		return fmt.Errorf("failed to read CA cert file: %w", err)
	}

	dialOpts, err := s.createRemoteClusterDialOption(certBytes, keyBytes, caBytes)
	if err != nil {
		return fmt.Errorf("failed to create dial options: %w", err)
	}

	conn, err := grpc.NewClient(s.host, dialOpts)
	if err != nil {
		return fmt.Errorf("failed to connect to Agones allocator: %w", err)
	}

	s.conn = conn
	s.client = pb.NewAllocationServiceClient(conn)

	return nil
}

func (s *AgonesStrategy) createRemoteClusterDialOption(clientCert, clientKey, caCert []byte) (grpc.DialOption, error) {
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	if len(caCert) != 0 {
		tlsConfig.RootCAs = x509.NewCertPool()
		if !tlsConfig.RootCAs.AppendCertsFromPEM(caCert) {
			return nil, pkgerrors.New("only PEM format is accepted for server CA")
		}
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

func (s *AgonesStrategy) Resolve(ctx context.Context, fqdn string) (string, error) {
	s.mu.RLock()
	fleetName, ok := s.fleets[fqdn]
	client := s.client
	enabled := s.enabled
	s.mu.RUnlock()

	if !ok {
		return "", errors.New("agones fleet not mapped for FQDN")
	}

	if !enabled || client == nil {
		return "", errors.New("agones strategy is not enabled or initialized")
	}

	return s.allocate(ctx, fleetName)
}

func (s *AgonesStrategy) UpdateRoute(fqdn, fleetName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fleets[fqdn] = fleetName
}

func (s *AgonesStrategy) allocate(ctx context.Context, fleetName string) (string, error) {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return "", errors.New("agones client not initialized")
	}

	request := &pb.AllocationRequest{
		Namespace: "default", // Should this be configurable?
		MultiClusterSetting: &pb.MultiClusterSetting{
			Enabled: false,
		},
		RequiredGameServerSelector: &pb.GameServerSelector{
			MatchLabels: map[string]string{
				"agones.dev/fleet": fleetName,
			},
		},
	}

	resp, err := client.Allocate(ctx, request)
	if err != nil {
		return "", fmt.Errorf("agones allocation failed: %w", err)
	}

	// Assuming we want to return "ip:port"
	return fmt.Sprintf("%s:%d", resp.Address, resp.Ports[0].Port), nil
}
