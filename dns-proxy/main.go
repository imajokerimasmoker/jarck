package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

var (
	listenAddr   = flag.String("listen", ":8053", "Listen address")
	upstreamAddr = flag.String("upstream", "8.8.8.8:53", "Upstream DNS server")
	timeout      = flag.Duration("timeout", 5*time.Second, "Timeout for upstream queries")
)

func main() {
	flag.Parse()

	handler := &DNSHandler{
		upstream: *upstreamAddr,
		client: &dns.Client{
			Net:     "udp",
			Timeout: *timeout,
		},
		tcpClient: &dns.Client{
			Net:     "tcp",
			Timeout: *timeout,
		},
	}

	dns.HandleFunc(".", handler.ServeDNS)

	go func() {
		srv := &dns.Server{Addr: *listenAddr, Net: "udp"}
		log.Printf("Starting UDP DNS proxy on %s, upstream %s", *listenAddr, *upstreamAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start UDP server: %s", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{Addr: *listenAddr, Net: "tcp"}
		log.Printf("Starting TCP DNS proxy on %s, upstream %s", *listenAddr, *upstreamAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start TCP server: %s", err.Error())
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("Shutting down...")
}

type DNSHandler struct {
	upstream  string
	client    *dns.Client
	tcpClient *dns.Client
}

func (h *DNSHandler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		dns.HandleFailed(w, r)
		return
	}

	q := r.Question[0]
	client := h.client
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		client = h.tcpClient
	}

	// Only handle A queries for rewrite
	if q.Qtype == dns.TypeA {
		log.Printf("Query: %s A -> rewriting to AAAA", q.Name)

		// 1. Try AAAA query
		aaaaQuery := r.Copy()
		aaaaQuery.Question[0].Qtype = dns.TypeAAAA

		resp, _, err := client.Exchange(aaaaQuery, h.upstream)
		if err == nil && resp != nil && len(resp.Answer) > 0 {
			log.Printf("Found AAAA records for %s", q.Name)
			resp.Id = r.Id
			resp.Question[0] = q // Keep original question
			w.WriteMsg(resp)
			return
		}

		if err != nil {
			log.Printf("AAAA query failed for %s: %v", q.Name, err)
		} else {
			log.Printf("No AAAA records for %s, falling back to A", q.Name)
		}
		// 2. Fallback to A query (which is the original query)
	}

	// Default forwarding
	resp, _, err := client.Exchange(r, h.upstream)
	if err != nil {
		log.Printf("Upstream error for %s: %v", q.Name, err)
		dns.HandleFailed(w, r)
		return
	}
	if resp != nil {
		resp.Id = r.Id
		w.WriteMsg(resp)
	}
}
