#!/usr/bin/env python3
"""
Remote Studio - eBPF Zero-Latency Connection Tracker (Experimental)
This requires root (CAP_BPF/CAP_TRACING) and the python3-bpfcc (bcc) package.

It intercepts tcp_v4_connect to instantly detect when an incoming connection 
reaches the RustDesk port (21118 by default), bypassing the need to poll `ss`.
"""

from bcc import BPF
import ctypes as ct
import sys
import struct
import socket

# RustDesk default TCP port
RUSTDESK_PORT = 21118

bpf_text = """
#include <uapi/linux/ptrace.h>
#include <net/sock.h>
#include <bcc/proto.h>

BPF_PERF_OUTPUT(ipv4_events);

struct ipv4_data_t {
    u32 saddr;
    u32 daddr;
    u16 lport;
    u16 dport;
};

int kretprobe__tcp_v4_connect(struct pt_regs *ctx) {
    struct sock *sk = (struct sock *)PT_REGS_PARM1(ctx);
    if (!sk) return 0;
    
    u16 dport = sk->sk_dport;
    dport = ntohs(dport);
    
    // Only track connections heading to our RustDesk port
    if (dport == %d) {
        struct ipv4_data_t data = {};
        data.saddr = sk->sk_rcv_saddr;
        data.daddr = sk->sk_daddr;
        data.lport = sk->sk_num;
        data.dport = dport;
        ipv4_events.perf_submit(ctx, &data, sizeof(data));
    }
    return 0;
}
""" % RUSTDESK_PORT

def print_ipv4_event(cpu, data, size):
    event = b["ipv4_events"].event(data)
    src_ip = socket.inet_ntoa(struct.pack("<L", event.saddr))
    dst_ip = socket.inet_ntoa(struct.pack("<L", event.daddr))
    print(f"eBPF Event: Fast connection detected from {dst_ip} -> {src_ip}:{event.dport}")
    # In a full implementation, this would trigger a D-Bus signal to the user daemon to instantly start the session.

if __name__ == "__main__":
    try:
        b = BPF(text=bpf_text)
    except Exception as e:
        print(f"Failed to load eBPF program: {e}")
        print("Note: This script must be run as root with the bcc toolchain installed.")
        sys.exit(1)

    print("Remote Studio eBPF Tracker attached to tcp_v4_connect.")
    print("Waiting for incoming RustDesk connections...")
    
    b["ipv4_events"].open_perf_buffer(print_ipv4_event)
    
    while True:
        try:
            b.perf_buffer_poll()
        except KeyboardInterrupt:
            exit()
