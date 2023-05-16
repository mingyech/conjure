import struct
from fcntl import ioctl
from scapy.all import *

def openTun(tunName):
    tun = open("/dev/net/tun", "r+b", buffering=0)
    LINUX_IFF_TUN = 0x0001
    LINUX_IFF_NO_PI = 0x1000
    LINUX_TUNSETIFF = 0x400454CA
    flags = LINUX_IFF_TUN | LINUX_IFF_NO_PI
    ifs = struct.pack("16sH22s", tunName, flags, b"")
    ioctl(tun, LINUX_TUNSETIFF, ifs)
    return tun


sport = 6789
src = '1.2.3.5'
dst = '5.6.7.9'
tun = openTun(b"tun1")
#pkt = IP(src='1.2.3.4', dst='5.6.7.8') / TCP(sport=1234, dport=443) / Raw(b'Hello world')
pkt = IP(src=src, dst=dst) / UDP(sport=sport, dport=443) / Raw(b'Hello world')
tun.write(bytes(pkt))

print('Wrote pkt')
resp = tun.read(1024)
print(resp)

pkt = IP(src=src, dst=dst) / UDP(sport=sport, dport=443) / Raw(b'Hi again')
tun.write(bytes(pkt))


print('Wrote again')
resp = tun.read(1024)
print(resp)
