package state

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestBypassRuleMatch(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("10.0.0.0/8")
	rules := NewBypassRuleSet([]BypassRule{{Protocol: 6, DstIPNets: []*net.IPNet{cidr}, DstPorts: []uint16{443}}})
	if !rules.Match(6, net.ParseIP("10.1.2.3"), 443) {
		t.Fatal("expected bypass rule to match")
	}
	if rules.Match(17, net.ParseIP("10.1.2.3"), 443) {
		t.Fatal("unexpected protocol match")
	}
}

func TestDNSNATAllocatesFakeIP(t *testing.T) {
	table := NewDNSNATTable()
	fake, err := table.GetOrCreate("game.example.com", net.ParseIP("203.0.113.10"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fake.String(); got != "198.18.0.1" {
		t.Fatalf("fake ip = %s", got)
	}
	real, ok := table.RealForFake(fake)
	if !ok || real.String() != "203.0.113.10" {
		t.Fatalf("real lookup failed: %v %v", real, ok)
	}
}

func TestKnownIPTableConcurrentAccess(t *testing.T) {
	table := NewKnownIPTable(make(chan struct{}, 1))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := net.IPv4(203, 0, 113, byte(i+1))
			table.Add(ip, nil)
			if !table.Contains(ip) {
				t.Errorf("missing %s", ip)
			}
		}(i)
	}
	wg.Wait()
	if len(table.All()) != 32 {
		t.Fatalf("known IP count = %d", len(table.All()))
	}
}

func TestPIDAndConnTables(t *testing.T) {
	pids := NewPIDTable()
	pids.Add(&PIDEntry{PID: 42, ProcessName: "game.exe", GameID: "game"})
	if entry, ok := pids.Get(42); !ok || entry.ProcessName != "game.exe" {
		t.Fatal("pid lookup failed")
	}

	conns := NewConnTrackTable()
	key := ConnKey{Protocol: 17, SrcPort: 5000, DstPort: 7000}
	conns.Upsert(&ConnEntry{Key: key, RealDstIP: net.ParseIP("203.0.113.20"), LastSeen: time.Now()})
	if entry, ok := conns.Get(key); !ok || entry.RealDstIP.String() != "203.0.113.20" {
		t.Fatal("conn lookup failed")
	}
}
