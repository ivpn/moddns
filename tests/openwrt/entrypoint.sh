#!/bin/sh
# Boots OpenWrt as a router on the compose network (dns_tests/test_openwrt.py).
set -eu

# Without this, netifd bridges eth0 into br-lan (192.168.1.1) and the container
# loses its compose address. No ISP resolver: dnsmasq resolves only via https-dns-proxy.
cat > /etc/config/network <<EOF
config interface 'loopback'
	option device 'lo'
	option proto 'static'
	option ipaddr '127.0.0.1'
	option netmask '255.0.0.0'

config interface 'lan'
	option device 'eth0'
	option proto 'static'
	option ipaddr '${LAN_IPADDR}'
	option netmask '255.255.0.0'
	option gateway '${LAN_GATEWAY}'
EOF

# No DHCP server or router advertisements on the shared compose network.
uci -q batch <<EOF
set dhcp.lan.ignore='1'
set dhcp.lan.dhcpv6='disabled'
set dhcp.lan.ra='disabled'
commit dhcp
EOF

# Trust for the proxy's dev certificate (https-dns-proxy uses libcurl's bundle).
cat /certs/dev-ca.crt >> /etc/ssl/certs/ca-certificates.crt

# Public resolvers (1.1.1.1, 8.8.8.8, ...) are played by knot, which knows the test zone.
# Plain DNS to stack addresses, the proxy included, and to the router itself is untouched.
nft -f - <<EOF
table ip e2e_internet {
	chain output {
		type nat hook output priority dstnat; policy accept;
		ip daddr != { ${STACK_SUBNET}, 127.0.0.0/8 } meta l4proto { tcp, udp } th dport 53 dnat to ${PUBLIC_DNS}
	}
}
EOF

exec /sbin/init
