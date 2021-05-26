import subprocess
from container_experiment import ContainerExperiment
from process_service import ProcessService
import os


# import time

def get_memory_usage(process_name: str, debug=False):
    qp = subprocess.Popen([
        'ps', '-C', process_name, '-o', 'pid', '--no-headers'
    ], preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    a, b = qp.communicate(timeout=5)
    a = a.decode("utf-8")
    b = b.decode("utf-8")

    if debug:
        print(a, end='')
        print(b, end='')

    a = a.strip()
    if b == "" and a.isdigit():
        pid = int(a)
    else:
        if debug:
            print("cannot parse pid for containerd")
        return

    memory = 0
    with open('/proc/%d/status' % pid, 'r') as f:
        for line in f:
            if line.startswith("VmHWM"):
                if debug:
                    print(line, end='')
                memory = [int(k) for k in line.split(' ') if k.isdigit()][0]
                break

    return memory


def get_cpu_usage(process_name: str, debug=False):
    qp = subprocess.Popen([
        'ps', '-C', process_name, '-o', 'cputimes', '--no-headers'
    ], preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    a, b = qp.communicate(timeout=5)
    a = a.decode("utf-8")
    b = b.decode("utf-8")

    if debug:
        print(a, end='')
        print(b, end='')

    a = a.strip()
    if b == "" and a.isdigit():
        return int(a)
    else:
        if debug:
            print("cannot parse pid for containerd")
        return 0


def get_cumulated_traffic(device: str, debug):
    nrx, ntx = 0, 0
    ifconfig_p = subprocess.Popen([
        'ifconfig', device
    ], preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    a, b = ifconfig_p.communicate(timeout=5)
    a = a.decode("utf-8")
    b = b.decode("utf-8")

    if debug:
        print(a, end='')
        print(b, end='')

    if b == "":
        for li in a.split('\n'):
            sli = li.strip()
            if sli.startswith('RX packets'):
                rx = sli.split(' ')[5]
                if rx.isdigit():
                    nrx = int(rx)
                pass
            if sli.startswith('TX packets'):
                tx = sli.split(' ')[5]
                if tx.isdigit():
                    ntx = int(tx)
                pass
        return nrx, ntx

    return 0, 0


class SystemInfo:
    def __init__(self, service: ProcessService):
        self.service = service

    def save_memory_usage(self,
                          experiment: ContainerExperiment,
                          method: str = "",
                          rtt: int = 0,
                          seq: int = 0,

                          event="done",

                          debug: bool = False):
        try:
            if self.service.p_containerd is not None:
                mem = get_memory_usage(self.service.config.CONTAINERD_GRPC, debug=debug)
                experiment.add_event(method, "mem-containerd-%s" % event, rtt, seq, delta=mem)
            else:
                if debug:
                    print("get memory containerd not exists")
        except:
            if debug:
                print("get containerd memory error")
            pass

        if method.startswith('starlight'):
            try:
                if self.service.p_starlight is not None:
                    mem = get_memory_usage(self.service.config.STARLIGHT_GRPC, debug=debug)
                    experiment.add_event(method, "mem-starlight-%s" % event, rtt, seq, delta=mem)
                else:
                    if debug:
                        print("get memory starlight not exists")
            except:
                if debug:
                    print("get starlight memory error")
                pass
            pass

        if method.startswith('estargz'):
            try:
                if self.service.p_stargz is not None:
                    mem = get_memory_usage(self.service.config.STARGZ_GRPC, debug=debug)
                    experiment.add_event(method, "mem-estargz-%s" % event, rtt, seq, delta=mem)
                else:
                    if debug:
                        print("get memory estargz not exists")
            except:
                if debug:
                    print("get estargz memory error")
                pass
            pass

    def save_cpu_usage(self,
                       experiment: ContainerExperiment,
                       method: str = "",
                       rtt: int = 0,
                       seq: int = 0,

                       event="done",

                       debug: bool = False):
        try:
            if self.service.p_containerd is not None:
                cpu_time = get_cpu_usage(self.service.config.CONTAINERD_GRPC, debug=debug)
                experiment.add_event(method, "cpu-containerd-%s" % event, rtt, seq, delta=cpu_time)
            else:
                if debug:
                    print("get cpu time containerd not exists")
        except:
            if debug:
                print("get containerd cpu time error")
            pass

        if method.startswith('starlight'):
            try:
                if self.service.p_starlight is not None:
                    cpu_time = get_cpu_usage(self.service.config.STARLIGHT_GRPC, debug=debug)
                    experiment.add_event(method, "cpu-starlight-%s" % event, rtt, seq, delta=cpu_time)
                else:
                    if debug:
                        print("get cpu time starlight not exists")
            except:
                if debug:
                    print("get starlight cpu time error")
                pass
            pass

        if method.startswith('estargz'):
            try:
                if self.service.p_stargz is not None:
                    cpu_time = get_cpu_usage(self.service.config.STARGZ_GRPC, debug=debug)
                    experiment.add_event(method, "cpu-estargz-%s" % event, rtt, seq, delta=cpu_time)
                else:
                    if debug:
                        print("get cpu time estargz not exists")
            except:
                if debug:
                    print("get estargz cpu time error")
                pass
            pass

    def save_traffic_usage(self,
                           experiment: ContainerExperiment,
                           method: str = "",
                           rtt: int = 0,
                           seq: int = 0,

                           event="done",

                           debug: bool = False):

        receive_n, sent_n = get_cumulated_traffic(self.service.config.NETWORK_DEVICE_WORKER, debug)
        experiment.add_event(method, "cum-receive-%s" % event, rtt, seq, delta=receive_n)
        experiment.add_event(method, "cum-sent-%s" % event, rtt, seq, delta=sent_n)
        pass


"""
if __name__ == '__main__':

    r = Runner()
    exp = ContainerExperiment('test', 'ready', '2', '1')
    hr = SystemInfo(r.service)

    r.service.reset_container_service()
    hr.save_memory_usage(
        exp,
        method='estargz',
        rtt=0,
        seq=0,
        event='done',
        debug=False
    )
    hr.save_traffic_usage(
        exp,
        method='estargz',
        rtt=0,
        seq=0,
        event='done',
        debug=False
    )

    time.sleep(10)
    hr.save_memory_usage(
        exp,
        method='estargz',
        rtt=0,
        seq=0,
        event='done',
        debug=True
    )
    hr.save_traffic_usage(
        exp,
        method='estargz',
        rtt=0,
        seq=0,
        event='done',
        debug=True
    )
    exp.save_event('-dev')
"""
