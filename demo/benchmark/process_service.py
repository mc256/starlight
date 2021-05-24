import subprocess
import time

import constants as config
from process_ctrl import kill_process, start_process_shell, start_process


class ProcessService:
    GRPC_PLUGIN_WAIT = 3

    def __init__(self):
        self.config = config.Configuration()
        self.p_stargz = None
        self.p_starlight = None
        self.p_containerd = None
        self.p_reset = None

    def reset_container_service(self, is_debug=False):
        self.p_reset = subprocess.Popen(
            ['sudo %s 2>&1%s' % (self.config.RESET, self.config.TEE_LOG_CONTAINERD)],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            shell=True
        )
        _ = self.p_reset.communicate()

        self.p_containerd = subprocess.Popen(
            ['sudo %s 2>&1%s' % (self.config.CONTAINERD, self.config.TEE_LOG_CONTAINERD)],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            shell=True
        )

        for ln in self.p_containerd.stdout:
            line = ln.decode('utf-8')
            if is_debug is True:
                print(line, end="")
            if line.find("containerd successfully booted") != -1:
                return

    def start_grpc_estargz(self):
        self.p_stargz = start_process_shell("sudo %s "
                                            "--address=/run/containerd-stargz-grpc/containerd-stargz-grpc.socket "
                                            "--config=/etc/containerd-stargz-grpc/config.toml "
                                            "--log-level=debug 2>&1%s" % (
                                                self.config.STARGZ_GRPC,
                                                self.config.TEE_LOG_STARGZ
                                            )
                                            )
        time.sleep(self.GRPC_PLUGIN_WAIT)
        return self.p_stargz

    def kill_estargz(self):
        kill_process(self.p_stargz)
        self.p_stargz = None

    def start_grpc_starlight(self):
        self.p_starlight = start_process_shell("sudo %s run "
                                               "--log-level=debug %s"
                                               "--server=%s "
                                               " 2>&1%s" % (
                                                   self.config.STARLIGHT_GRPC,
                                                   "" if self.config.USE_HTTPS else "--plain-http ",
                                                   self.config.PROXY_SERVER,
                                                   self.config.TEE_LOG_STARLIGHT
                                               )
                                               )
        time.sleep(self.GRPC_PLUGIN_WAIT)
        return self.p_starlight

    def kill_starlight(self):
        kill_process(self.p_starlight)
        self.p_starlight = None

    def start_all_grpc(self):
        return self.start_grpc_estargz(), self.start_grpc_starlight()

    def set_latency_bandwidth(self, rtt, bandwidth=100, debug=False):
        p_worker = start_process([
            "sudo", "tc", "qdisc", "add", "dev", self.config.NETWORK_DEVICE_WORKER,
            "root", "netem", "delay", "%.1fms" % (rtt / 2), "rate", "%dMbit" % bandwidth
        ])
        p_registry = start_process(self.config.SSH_TO_REGISTRY + [
            "sudo", "tc", "qdisc", "add", "dev", self.config.NETWORK_DEVICE_REGISTRY,
            "root", "netem", "delay", "%.1fms" % (rtt / 2), "rate", "%dMbit" % bandwidth
        ])
        p_proxy = start_process(self.config.SSH_TO_STARLIGHT_PROXY + [
            "sudo", "tc", "qdisc", "add", "dev", self.config.NETWORK_DEVICE_STARLIGHT_PROXY,
            "root", "netem", "delay", "%.1fms" % (rtt / 2), "rate", "%dMbit" % bandwidth
        ])
        if debug is True:
            for ln in p_worker.stdout:
                print(ln)
            for ln in p_registry.stdout:
                print(ln)
            for ln in p_proxy.stdout:
                print(ln)
        p_worker.wait()
        p_registry.wait()
        p_proxy.wait()

    def reset_latency_bandwidth(self, debug=False):
        p_worker = start_process([
            "sudo", "tc", "qdisc", "del", "dev", self.config.NETWORK_DEVICE_WORKER, "root"
        ])
        p_registry = start_process(self.config.SSH_TO_REGISTRY + [
            "sudo", "tc", "qdisc", "del", "dev", self.config.NETWORK_DEVICE_REGISTRY, "root"
        ])
        p_proxy = start_process(self.config.SSH_TO_STARLIGHT_PROXY + [
            "sudo", "tc", "qdisc", "del", "dev", self.config.NETWORK_DEVICE_STARLIGHT_PROXY, "root"
        ])
        if debug is True:
            for ln in p_worker.stdout:
                print(ln)
            for ln in p_registry.stdout:
                print(ln)
            for ln in p_proxy.stdout:
                print(ln)
        p_worker.wait()
        p_registry.wait()
        p_proxy.wait()
