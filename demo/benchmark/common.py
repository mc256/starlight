import subprocess, os
import time

GRPC_PLUGIN_WAIT = 3


def reset_container_service():
    p = subprocess.Popen(
        ['sudo ./reset.sh 2>&1'],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        shell=True
    )

    for ln in p.stdout:
        line = ln.decode('utf-8')
        if line.find("containerd successfully booted") != -1:
            return


def terminate_process(p):
    pgid = os.getpgid(p.pid)
    subprocess.Popen(["sudo", "kill", "-s", "15", "%d" % pgid])
    p.wait()


def kill_process(p):
    pgid = os.getpgid(p.pid)
    subprocess.Popen(["sudo", "kill", "-s", "9", "%d" % pgid])
    p.wait()


def start_process_shell(args):
    pp = subprocess.Popen(args, preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True)
    return pp


def start_process(args):
    pp = subprocess.Popen(args, preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    return pp


def call_wait(args, out=False):
    pr = subprocess.Popen(args, preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if out is True:
        for ln in pr.stdout:
            print(ln)
    pr.wait()


######################################################################################

def start_grpc_estargz(cfg):
    stargz_p = start_process_shell("sudo %s "
                                   "--address=/run/containerd-stargz-grpc/containerd-stargz-grpc.socket "
                                   "--config=/etc/containerd-stargz-grpc/config.toml "
                                   "--log-level=debug 2>&1" % cfg.STARGZ_GRPC
                                   )
    time.sleep(GRPC_PLUGIN_WAIT)
    return stargz_p


def start_grpc1(cfg):
    return start_grpc_estargz(cfg)


def start_grpc_starlight(cfg):
    starlight_p = start_process_shell("sudo %s run "
                                      "--log-level=debug "
                                      "--plain-http "
                                      "--server=%s:8090 "
                                      " 2>&1"
                                      " " % (cfg.STARLIGHT_GRPC, cfg.PROXY_SERVER)
                                      )
    time.sleep(GRPC_PLUGIN_WAIT)
    return starlight_p


def start_grpc2(cfg):
    return start_grpc_starlight(cfg)


def start_all_grpc(cfg):
    stargz_p = start_grpc1(cfg)
    starlight_p = start_grpc2(cfg)
    return stargz_p, starlight_p



def set_latency_bandwidth(cfg, latency, bandwidth = 100, debug=False):
    p1 = start_process([
        "sudo", "tc", "qdisc", "add", "dev", cfg.NETWORK_DEVICE,
        "root", "netem", "delay", "%dms" % (latency // 2), "rate", "%dMbit" % bandwidth
    ])
    p2 = start_process([
        "ssh", "maverick@cloudy",
        "sudo tc qdisc add dev enp3s0 root netem delay %dms rate 100Mbit" % (latency //2)
    ])
    p3 = start_process([
        "ssh", "maverick@starlight",
        "sudo tc qdisc add dev enp4s0 root netem delay %dms rate 100Mbit" % (latency //2)
    ])
    if debug is True:
        for ln in p1.stdout:
            print(ln)
        for ln in p2.stdout:
            print(ln)
        for ln in p3.stdout:
            print(ln)
    p1.wait()
    p2.wait()
    p3.wait()


def reset_latency_bandwidth(cfg, debug=False):
    p1 = start_process(["sudo","tc","qdisc","del","dev",cfg.NETWORK_DEVICE,"root"])
    p2 = start_process([
        "ssh", "maverick@cloudy",
        "sudo tc qdisc del dev enp3s0 root"
    ])
    p3 = start_process([
        "ssh", "maverick@starlight",
        "sudo tc qdisc del dev enp4s0 root"
    ])
    if debug is True:
        for ln in p1.stdout:
            print(ln)
        for ln in p2.stdout:
            print(ln)
        for ln in p3.stdout:
            print(ln)
    p1.wait()
    p2.wait()
    p3.wait()
