package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

const (
	MediaTypeFormatAndVersion = "application/external.dns.webhook+json;version=1"
	ContentTypeHeader         = "Content-Type"
)

// Provider defines the interface that the webhook server needs.
type Provider interface {
	Records(ctx context.Context) ([]*endpoint.Endpoint, error)
	ApplyChanges(ctx context.Context, changes *plan.Changes) error
	AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error)
	GetDomainFilter() endpoint.DomainFilter
}

// WebhookServer handles the HTTP API for the webhook provider.
type WebhookServer struct {
	Provider Provider
}

func (s *WebhookServer) NegotiateHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(ContentTypeHeader, MediaTypeFormatAndVersion)
	if err := json.NewEncoder(w).Encode(s.Provider.GetDomainFilter()); err != nil {
		log.Errorf("Failed to encode domain filter: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *WebhookServer) RecordsHandler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		records, err := s.Provider.Records(req.Context())
		if err != nil {
			log.Errorf("Failed to get records: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set(ContentTypeHeader, MediaTypeFormatAndVersion)
		if err := json.NewEncoder(w).Encode(records); err != nil {
			log.Errorf("Failed to encode records: %v", err)
		}
	case http.MethodPost:
		var changes plan.Changes
		if err := json.NewDecoder(req.Body).Decode(&changes); err != nil {
			log.Errorf("Failed to decode changes: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := s.Provider.ApplyChanges(req.Context(), &changes); err != nil {
			log.Errorf("Failed to apply changes: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		log.Errorf("Unsupported method %s", req.Method)
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (s *WebhookServer) AdjustEndpointsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		log.Errorf("Unsupported method %s", req.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var endpoints []*endpoint.Endpoint
	if err := json.NewDecoder(req.Body).Decode(&endpoints); err != nil {
		log.Errorf("Failed to decode endpoints: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	adjusted, err := s.Provider.AdjustEndpoints(endpoints)
	if err != nil {
		log.Errorf("Failed to adjust endpoints: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set(ContentTypeHeader, MediaTypeFormatAndVersion)
	if err := json.NewEncoder(w).Encode(adjusted); err != nil {
		log.Errorf("Failed to encode adjusted endpoints: %v", err)
	}
}

// StartProviderServer starts the webhook provider HTTP server on providerAddr.
func StartProviderServer(p Provider, providerAddr string) error {
	s := &WebhookServer{Provider: p}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.NegotiateHandler)
	mux.HandleFunc("/records", s.RecordsHandler)
	mux.HandleFunc("/adjustendpoints", s.AdjustEndpointsHandler)

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	l, err := net.Listen("tcp", providerAddr)
	if err != nil {
		return err
	}

	log.Infof("Starting webhook provider server on %s", providerAddr)
	return srv.Serve(l)
}

// StartHealthServer starts the health check HTTP server on healthAddr.
func StartHealthServer(healthAddr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	l, err := net.Listen("tcp", healthAddr)
	if err != nil {
		return err
	}

	log.Infof("Starting health server on %s", healthAddr)
	return srv.Serve(l)
}
