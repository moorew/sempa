// Package netguard decides which addresses outbound fetches made on behalf of
// users or external content may reach. One predicate, shared by every guarded
// fetcher, so the unfurl and iCal guards can't drift apart again.
package netguard

import (
	"net"
	"net/netip"
)

// nonPublic lists special-use ranges that the stdlib predicates don't cover.
// 100.64.0.0/10 matters most here: it is carrier-grade NAT and every Tailscale
// node address, and Sempa usually runs on a tailnet.
var nonPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),   // CGNAT / Tailscale (incl. 100.100.100.100)
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved, incl. broadcast
	netip.MustParsePrefix("64:ff9b::/96"),    // NAT64 (maps onto IPv4)
	netip.MustParsePrefix("64:ff9b:1::/48"),  // local-use NAT64
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
}

// IsNonPublic reports whether ip is loopback, private, link-local, unspecified,
// multicast, or in another range a server-side fetch must never reach.
func IsNonPublic(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, p := range nonPublic {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
