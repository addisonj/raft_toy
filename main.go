package main

import (
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spacemonkeygo/spacelog"
)

var (
	bind      = flag.String("bind", "127.0.0.1:4000", "address to bind on for raft")
	seeds     = flag.String("seed", "", "list of nodes to connect to (implies bootstrap)")
	addr      = flag.String("addr", ":5000", "addres to listen on")
	bootstrap = flag.Bool("bootstrap", false, "bootstrap the nodes")
	dir       = flag.String("dir", "", "directory to store data in")
)

type Config struct {
	Port      int
	Bootstrap bool
}

func main() {
	flag.Parse()
	var seedList []string = nil
	var peerList []net.Addr
	logger := spacelog.GetLogger()
	if len(*seeds) > 0 {
		logger.Notice("bootstrapping node with seeds")
		seedList = strings.Split(*seeds, ",")
		for _, seed := range seedList {
			np, err := net.ResolveTCPAddr("tcp", seed)
			if err != nil {
				logger.Crit("probably have a bad seed in the list")
				logger.Crite(err)
				os.Exit(1)
			}
			if np != nil {
				peerList = append(peerList, np)
			}
		}
		*bootstrap = true
	}
	spacelog.SetLevel(nil, 10)
	logger.Notice("starting up this thing")
	logger.Notice(logger.DebugEnabled())
	node, err := StartNode(*bootstrap, *bind, peerList, *dir, logger)
	if err != nil {
		logger.Crite(err)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigc
		node.Stop()
		os.Exit(0)
	}()
	http.ListenAndServe(*addr, CreateServer("127.0.0.1:8080", node, logger))
	logger.Notice("exiting")
}
