package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/spacemonkeygo/spacelog"
)

var safeURL = regexp.MustCompile(`\A/[a-fA-F0-9]+\z`)

type updateBody struct {
	Body      string `json:"body,omitempty"`
	PrevValue string `json:"prevValue,omitempty"`
	NotExist  bool   `json:"notExist,omitempty"`
}

type Server struct {
	Node     *Node
	Upstream string
	logger   *spacelog.Logger
}

func CreateServer(upstream string, node *Node, logger *spacelog.Logger) http.HandlerFunc {
	s := &Server{node, upstream, logger}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Serve(w, r)
	})
}

// requires a JSON obj that contains the prevValue or notExist, which is a hash, or
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var update updateBody
	err := decoder.Decode(&update)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
	}
	obj_id := strings.ToLower(r.URL.Path[1:])
	err = s.Node.Lock(obj_id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
	}

	defer s.Node.Unlock(obj_id)
}

// block if there is a lock outstanding
func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {

}

func (s *Server) Serve(w http.ResponseWriter, r *http.Request) {
	if !s.Node.IsLeader() {
		// forward request
		http.Error(w, "need to forward request to leader", 400)
		return
	}
	if !safeURL.MatchString(r.URL.Path) {
		http.Error(w, fmt.Sprintf("invalid path %v", r.URL.Path), 400)
		return
	}
	switch r.Method {
	case "GET":
		s.handleRead(w, r)
	case "POST":
		s.handleUpdate(w, r)
	default:
		http.Error(w, fmt.Sprintf("invalid method %v", r.Method), 400)
	}
}
