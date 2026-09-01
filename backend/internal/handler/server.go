package handler

import "github.com/brenonaraujo/canteiro/backend/internal/repository"

// Server implements api.ServerInterface for F0 ops endpoints.
type Server struct {
	Service  string
	Checkers []repository.Checker
}

// NewServer returns an ops adapter.
func NewServer(service string, checkers []repository.Checker) *Server {
	if service == "" {
		service = "canteiro"
	}
	return &Server{Service: service, Checkers: checkers}
}
