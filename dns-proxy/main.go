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

		// 1. Try AAAA query with CNAME chasing
		resp, err := h.resolveWithCNAMEChasing(q.Name, dns.TypeAAAA, client, r.Id)
		if err == nil && resp != nil && h.hasRecordType(resp.Answer, dns.TypeAAAA) {
			log.Printf("Found AAAA records for %s (possibly via CNAME)", q.Name)
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

	// Default forwarding with CNAME chasing
	resp, err := h.resolveWithCNAMEChasing(q.Name, q.Qtype, client, r.Id)
	if err != nil {
		log.Printf("Upstream error for %s: %v", q.Name, err)
		dns.HandleFailed(w, r)
		return
	}
	if resp != nil {
		w.WriteMsg(resp)
	}
}

func (h *DNSHandler) resolveWithCNAMEChasing(name string, qtype uint16, client *dns.Client, id uint16) (*dns.Msg, error) {
	var finalResp *dns.Msg
	currentName := name
	maxChases := 5
	allAnswers := []dns.RR{}
	seenNames := make(map[string]bool)

	for i := 0; i < maxChases; i++ {
		if seenNames[dns.Fqdn(currentName)] {
			// Loop detected
			break
		}
		seenNames[dns.Fqdn(currentName)] = true

		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(currentName), qtype)
		m.RecursionDesired = true

		resp, _, err := client.Exchange(m, h.upstream)
		if err != nil {
			if finalResp != nil {
				finalResp.Answer = allAnswers
				finalResp.Id = id
				return finalResp, nil
			}
			return nil, err
		}

		if finalResp == nil {
			finalResp = resp
		}

		// Add answers that we don't already have
		for _, rr := range resp.Answer {
			isDup := false
			for _, existing := range allAnswers {
				if dns.IsDuplicate(existing, rr) {
					isDup = true
					break
				}
			}
			if !isDup {
				allAnswers = append(allAnswers, rr)
			}
		}

		// Check if we found the desired record type
		if h.hasRecordType(resp.Answer, qtype) {
			break
		}

		// Check for CNAME
		cname := h.getCNAME(resp.Answer, currentName)
		if cname == "" {
			// No more CNAMEs and no desired records found
			break
		}

		// Follow the CNAME
		currentName = cname
	}

	if finalResp == nil {
		return nil, nil
	}

	finalResp.Answer = allAnswers
	finalResp.Id = id
	return finalResp, nil
}

func (h *DNSHandler) hasRecordType(answers []dns.RR, qtype uint16) bool {
	for _, rr := range answers {
		if rr.Header().Rrtype == qtype {
			return true
		}
	}
	return false
}

func (h *DNSHandler) getCNAME(answers []dns.RR, name string) string {
	for _, rr := range answers {
		if cname, ok := rr.(*dns.CNAME); ok {
			if dns.Fqdn(rr.Header().Name) == dns.Fqdn(name) {
				return cname.Target
			}
		}
	}
	return ""
}
