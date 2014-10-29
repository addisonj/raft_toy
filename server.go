package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
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
		return
	}
	obj_id := strings.ToLower(r.URL.Path[1:])
	err = s.Node.Lock(obj_id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
		return
	}
	defer s.Node.Unlock(obj_id)

	resp, err := http.Head(buildUrl(s.Upstream, obj_id))
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
		return
	}
	if resp.StatusCode == 404 {
		if !update.NotExist {
			http.Error(w, "Specified value not existing, but does", 400)
			return
		}
	} else if resp.StatusCode == 200 {
		if resp.Header.Get("X-sha1") != update.PrevValue {
			http.Error(w, "PrevValue doesn't match current value", 400)
			return
		}
	}
	data, err := base64.StdEncoding.DecodeString(update.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
		return
	}
	pResp, err := http.Post(buildUrl(s.Upstream, obj_id), "multipart/upload", bytes.NewBuffer(data))
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
		return
	}
	if pResp.StatusCode != 200 {
		http.Error(w, "upstream gave non 200 response", 500)
		return
	}

	pResp.Write(w)
}

// block if there is a lock outstanding
func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	obj_id := strings.ToLower(r.URL.Path[1:])
	err := s.Node.Lock(obj_id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Bad Request: %v", err.Error()), 500)
	}
	defer s.Node.Unlock(obj_id)

	resp, err := http.Get(buildUrl(s.Upstream, obj_id))
	resp.Write(w)
}

func buildUrl(upstream, path string) string {
	return filepath.Join(upstream, path)
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
