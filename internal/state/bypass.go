package state

import "net"

type BypassRuleSet struct {
	rules []BypassRule
}

type BypassRule struct {
	Protocol  uint8
	DstIPNets []*net.IPNet
	DstPorts  []uint16
	Comment   string
}

func NewBypassRuleSet(rules []BypassRule) *BypassRuleSet {
	cp := append([]BypassRule(nil), rules...)
	return &BypassRuleSet{rules: cp}
}

func (s *BypassRuleSet) Match(protocol uint8, dstIP net.IP, dstPort uint16) bool {
	if s == nil {
		return false
	}
	for i := range s.rules {
		if s.rules[i].Match(protocol, dstIP, dstPort) {
			return true
		}
	}
	return false
}

func (s *BypassRuleSet) Rules() []BypassRule {
	if s == nil {
		return nil
	}
	return append([]BypassRule(nil), s.rules...)
}

func (r *BypassRule) Match(protocol uint8, dstIP net.IP, dstPort uint16) bool {
	if r.Protocol != 0 && r.Protocol != protocol {
		return false
	}
	if len(r.DstIPNets) > 0 {
		matched := false
		for _, ipnet := range r.DstIPNets {
			if ipnet != nil && ipnet.Contains(dstIP) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(r.DstPorts) > 0 {
		matched := false
		for _, p := range r.DstPorts {
			if p == dstPort {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
