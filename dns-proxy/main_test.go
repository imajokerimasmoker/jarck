package main

import (
	"testing"

	"github.com/miekg/dns"
)

func TestHasRecordType(t *testing.T) {
	h := &DNSHandler{}
	answers := []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA}, A: []byte{1, 2, 3, 4}},
	}

	if !h.hasRecordType(answers, dns.TypeA) {
		t.Errorf("Expected to find TypeA")
	}

	if h.hasRecordType(answers, dns.TypeAAAA) {
		t.Errorf("Did not expect to find TypeAAAA")
	}
}

func TestGetCNAME(t *testing.T) {
	h := &DNSHandler{}
	answers := []dns.RR{
		&dns.CNAME{Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeCNAME}, Target: "target.com."},
	}

	target := h.getCNAME(answers, "example.com.")
	if target != "target.com." {
		t.Errorf("Expected target.com., got %s", target)
	}

	target2 := h.getCNAME(answers, "other.com.")
	if target2 != "" {
		t.Errorf("Expected empty string for non-matching name, got %s", target2)
	}
}
