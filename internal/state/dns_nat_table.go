package state

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
)

var ErrFakeIPRangeExhausted = errors.New("fake IP range exhausted")

type DNSNATTable struct {
	fakeToReal   map[string]string
	realToFake   map[string]string
	domainToFake map[string]string
	mu           sync.RWMutex
	nextFakeIP   uint32
}

func NewDNSNATTable() *DNSNATTable {
	return &DNSNATTable{
		fakeToReal:   make(map[string]string),
		realToFake:   make(map[string]string),
		domainToFake: make(map[string]string),
		nextFakeIP:   binary.BigEndian.Uint32(net.IPv4(198, 18, 0, 1).To4()),
	}
}

func (t *DNSNATTable) GetOrCreate(domain string, realIP net.IP) (net.IP, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if fake, ok := t.domainToFake[domain]; ok {
		return net.ParseIP(fake), nil
	}
	if realIP != nil {
		if fake, ok := t.realToFake[realIP.String()]; ok {
			t.domainToFake[domain] = fake
			return net.ParseIP(fake), nil
		}
	}
	fake, err := t.allocateLocked()
	if err != nil {
		return nil, err
	}
	fakeStr := fake.String()
	t.domainToFake[domain] = fakeStr
	if realIP != nil {
		realStr := realIP.String()
		t.fakeToReal[fakeStr] = realStr
		t.realToFake[realStr] = fakeStr
	}
	return fake, nil
}

func (t *DNSNATTable) UpdateReal(fakeIP net.IP, realIP net.IP) {
	if fakeIP == nil || realIP == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	fakeStr := fakeIP.String()
	realStr := realIP.String()
	t.fakeToReal[fakeStr] = realStr
	t.realToFake[realStr] = fakeStr
}

func (t *DNSNATTable) RealForFake(fakeIP net.IP) (net.IP, bool) {
	if fakeIP == nil {
		return nil, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	real, ok := t.fakeToReal[fakeIP.String()]
	return net.ParseIP(real), ok
}

func (t *DNSNATTable) FakeForReal(realIP net.IP) (net.IP, bool) {
	if realIP == nil {
		return nil, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	fake, ok := t.realToFake[realIP.String()]
	return net.ParseIP(fake), ok
}

func (t *DNSNATTable) allocateLocked() (net.IP, error) {
	end := binary.BigEndian.Uint32(net.IPv4(198, 19, 255, 254).To4())
	if t.nextFakeIP > end {
		return nil, ErrFakeIPRangeExhausted
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, t.nextFakeIP)
	t.nextFakeIP++
	return net.IPv4(buf[0], buf[1], buf[2], buf[3]), nil
}
