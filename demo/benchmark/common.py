import signal
import subprocess, os
import time
import random
import constants as config
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import re
from datetime import date


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


class ContainerExperiment:
    # STARGZ_SUFFIX = "-stargz"
    STARGZ_SUFFIX = "-starlight"
    STARLIGHT_SUFFIX = "-starlight"

    def __init__(self, image_name, ready_keyword, version, old_version):
        self.ready_keyword = ready_keyword
        self.image_name = image_name
        self.version = version
        self.old_version = old_version
        self.has_mounting = False
        # self.rtt = [2, 25, 50, 75, 100, 125, 150, 175, 200, 225, 250, 275, 300]
        self.rtt = [2, 50, 100, 150, 200, 250, 300]
        self.rounds = 20
        self.expected_max_start_time = 30
        self.mounting = []
        self.has_args = False
        self.args = []

        self.experiment_name = ""
        self.update_experiment_name()

        self.action_history = []

    def set_experiment_name(self, name):
        self.experiment_name = name

    def update_experiment_name(self):
        today = date.today().strftime("%m%d")
        if self.old_version == "":
            self.experiment_name = "%s-%s--deploy-%s-r%d" % (self.image_name, today, self.version, self.rounds)
        else:
            self.experiment_name = "%s-%s--%s_%s-r%d" % (
                self.image_name, today, self.version, self.old_version, self.rounds)

    def set_mounting_points(self, mp=None):
        if mp is None:
            return
        self.mounting = mp
        self.has_mounting = True

    def set_args(self, args=None):
        if args is None:
            return
        self.has_args = True
        self.args = args

    def get_starlight_image(self, old=False):
        if old:
            if self.old_version == "":
                raise AssertionError("It should have an old image")
            return "%s:%s%s" % (self.image_name, self.old_version, self.STARLIGHT_SUFFIX)
        else:
            return "%s:%s%s" % (self.image_name, self.version, self.STARLIGHT_SUFFIX)

    def get_stargz_image(self, old=False):
        if old:
            if self.old_version == "":
                raise AssertionError("It should have an old image")
            return "%s:%s%s" % (self.image_name, self.old_version, self.STARGZ_SUFFIX)
        else:
            return "%s:%s%s" % (self.image_name, self.version, self.STARGZ_SUFFIX)

    def get_vanilla_image(self, old=False):
        if old:
            if self.old_version == "":
                raise AssertionError("It should have an old image")
            return "%s:%s" % (self.image_name, self.old_version)
        else:
            return "%s:%s" % (self.image_name, self.version)

    def has_old_version(self):
        return self.old_version != ""

    def load_results(self, suffix="", data_path="./pkl"):
        df1 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 1))
        df2 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 2))
        df3 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 3))
        df4 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 4))
        return df1, df2, df3, df4  # vanilla, estargz, starlight, wget

    def add_event(self, method="", event="", rtt=0, round=0, ts=time.time(), delta=0.0):
        self.action_history.append([method, event, rtt, round, ts, delta])

    def save_event(self, suffix=""):
        ddf = pd.DataFrame(self.action_history, columns=['method', 'event', 'rtt', 'round', 'ts', 'delta'])
        ddf.to_csv("./csv/%s%s-bundle.csv" % (self.experiment_name, suffix))

    def save_results(self, performance_estargz, performance_starlight, performance_vanilla, performance_wget,
                     position=1, suffix=""):
        estargz_np = np.array(performance_estargz)
        starlight_np = np.array(performance_starlight)
        vanilla_np = np.array(performance_vanilla)
        wget_np = np.array(performance_wget)

        df1 = pd.DataFrame(vanilla_np.T, columns=self.rtt[:position])
        df2 = pd.DataFrame(estargz_np.T, columns=self.rtt[:position])
        df3 = pd.DataFrame(starlight_np.T, columns=self.rtt[:position])
        df4 = pd.DataFrame(wget_np.T, columns=self.rtt[:position])

        df1.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 1))
        df2.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 2))
        df3.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 3))
        df4.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 4))

        return df1, df2, df3, df4

    def plot_single_result(self, step_starlight, step_vanilla, step_estargz, step_wget, suffix=""):
        df_avg = pd.DataFrame({
            "estargz": step_estargz,
            "starlight": step_starlight,
            "vanilla": step_vanilla,
            "wget": step_wget,
        })

        fig, (ax1) = plt.subplots(ncols=1, figsize=(4, 4), dpi=300)

        max_delay = self.expected_max_start_time

        fig.suptitle("%s%s" % (self.experiment_name, suffix))
        ax1.set_ylim([0, max_delay])
        ax1.set_ylabel('startup time (s)')
        ax1.set_xlabel('method')

        df_avg.plot(kind='box', ax=ax1, grid=True)
        ax1.title.set_text("fixed latency")

        fig.tight_layout()
        fig.savefig("./plot/%s%s.png" % (self.experiment_name, suffix), facecolor='w', transparent=False)

        plt.close(fig)

    def plot_results(self, df1, df2, df3, df4, suffix=""):
        df_avg = pd.DataFrame({
            'vanilla': df1.mean(),
            'estargz': df2.mean(),
            'starlight': df3.mean(),
            'wget': df4.mean(),
        },
            index=self.rtt
        )

        df1_q = df1.quantile([0.1, 0.9])
        df2_q = df2.quantile([0.1, 0.9])
        df3_q = df3.quantile([0.1, 0.9])
        df4_q = df4.quantile([0.1, 0.9])

        max_delay = self.expected_max_start_time

        fig, (ax1) = plt.subplots(ncols=1, figsize=(4, 4), dpi=300)

        fig.suptitle("%s%s" % (self.experiment_name, suffix))
        ax1.set_xlim([0, 300])
        ax1.set_ylim([0, max_delay])
        ax1.set_ylabel('startup time (s)')
        ax1.set_xlabel('RTT (ms)')

        ax1.fill_between(df1_q.columns, df1_q.loc[0.1], df1_q.loc[0.9], alpha=0.25)
        ax1.fill_between(df2_q.columns, df2_q.loc[0.1], df2_q.loc[0.9], alpha=0.25)
        ax1.fill_between(df3_q.columns, df3_q.loc[0.1], df3_q.loc[0.9], alpha=0.25)
        ax1.fill_between(df4_q.columns, df4_q.loc[0.1], df4_q.loc[0.9], alpha=0.25)

        df_avg.plot(kind='line', ax=ax1, grid=True, marker=".")
        ax1.legend(loc='upper left')
        ax1.title.set_text("mean & quantile[0.1,0.9]")

        fig.tight_layout()
        fig.savefig("./plot/%s%s.png" % (self.experiment_name, suffix), facecolor='w', transparent=False)

        plt.close(fig)

    def __repr__(self):
        return "ContainerExperiment<%s>" % self.experiment_name


class ContainerExperimentX(ContainerExperiment):
    def __init__(self, img_name, img_type, download, tag, tag_old, mounts, ready, args=None, expectation=60):
        """
        Create and experiment configuration
        Parameters
        ----------
        img_name : str
            Number of rows/columns of the subplot grid.

        img_type: str
            Category of the Image, based on HelloBench [FAST'16] or your paper. Not used

        download : str
            How many downloads from. Not used

        tag : str
            Version of the new container image. It should have no optimized suffix. Out program will add that
            (i.e. -starlight or -estargz) for you.

        tag_old : str
            Same as tag but an older version.

        mounts: List[MountingPoint]
            mounting points, the experiment will prepare external mounting point and remove those folder once finished.

        ready: str
            the line of text where container tells us it is ready.

        args: List[str] or None
            the parameters that can be executed.

        expectation: int
            expect when it will finish for the upper limit of the y-axis in plotting

        """
        super().__init__(
            img_name,
            ready,
            tag,
            tag_old
        )
        self.set_mounting_points(mounts)
        self.expected_max_start_time = expectation
        self.set_args(args)


class MountingPoint:
    WORKDIR = "/tmp/starlight-exp"

    def __init__(self, guest_dst, is_file=False, op_type="rw", owner="", overwrite=""):
        self.is_file = is_file
        self.guest_dst = guest_dst
        self.op_type = op_type
        self.owner = owner
        self.r = random.randrange(999999)
        self.overwrite = overwrite

    def reset_tmp(self, debug=False):
        p = start_process([
            "sudo", "rm", "-rf", "%s" % self.WORKDIR
        ])
        if debug is True:
            for ln in p.stdout:
                print(ln)
        p.wait()

    def prepare(self, rr=0, debug=False):
        p = start_process([
            "sudo", "mkdir", "-p", "%s/m%d-%d" % (self.WORKDIR, self.r, rr)
        ])
        if debug is True:
            for ln in p.stdout:
                print(ln)
        p.wait()

        if self.owner != "":
            p = start_process([
                "sudo", "chown", "-R", self.owner, "%s/m%d-%d" % (self.WORKDIR, self.r, rr)
            ])
            if debug is True:
                for ln in p.stdout:
                    print(ln)
            p.wait()

    def destroy(self, rr=0, debug=False):
        p = start_process([
            "sudo", "rm", "-rf", "%s/m%d-%d" % (self.WORKDIR, self.r, rr)
        ])
        if debug is True:
            for ln in p.stdout:
                print(ln)
        p.wait()

    def get_mount_parameter(self, rr=0):
        if self.overwrite != "":
            return self.overwrite
        return "type=bind,src=%s/m%d-%d,dst=%s,options=rbind:%s" % (
            self.WORKDIR, self.r, rr, self.guest_dst, self.op_type)


class Runner:
    def __init__(self):
        self.ycsb_p = None
        self.service = ProcessService()
        self.ycsb_base = ""
        self.timeout = 120
        pass

    def sync_pull_estargz(self, experiment: ContainerExperiment, r=0, debug=False):
        if r == 0:
            r = random.randrange(999999)

        spe_p = start_process_shell([
            "sudo ctr-remote -n xe%d image rpull %s %s/%s  2>&1" % (
                r,
                "" if self.service.config.USE_HTTPS else "--plain-http ",
                self.service.config.REGISTRY_SERVER,
                experiment.get_stargz_image(old=True)
            )
        ])
        spe_p.wait()

        complete = 0
        for ln in self.service.p_stargz.stdout:
            line = ln.decode('utf-8')
            if debug:
                print(line, end="")
            if line.find("resolving") != -1:
                complete += 1
            if line.find("completed to fetch all layer data in background") != -1:
                complete -= 1
                if complete == 0:
                    break

        time.sleep(1)
        return r

    def sync_pull_starlight(self, experiment: ContainerExperiment, r=0, debug=False):
        if r == 0:
            r = random.randrange(999999)

        sps_p = start_process_shell([
            "sudo ctr-starlight -n xs%d prepare %s 2>&1" % (
                r,
                experiment.get_starlight_image(old=True)
            )
        ])
        sps_p.wait()

        for ln in self.service.p_starlight.stdout:
            line = ln.decode('utf-8')
            if debug:
                print(line, end="")
            if line.find("entire image extracted") != -1:
                break

        time.sleep(1)
        return r

    def sync_pull_vanilla(self, experiment: ContainerExperiment, r=0, debug=False):
        if r == 0:
            r = random.randrange(999999)

        pull = start_process_shell([
            "sudo ctr -n xv%d image pull %s %s/%s 2>&1" % (
                r,
                "" if self.service.config.USE_HTTPS else "--plain-http ",
                self.service.config.REGISTRY_SERVER,
                experiment.get_vanilla_image(old=True)
            )
        ])
        last_line = ""
        for ln in pull.stdout:
            line = ln.decode('utf-8')
            last_line = line
            pass
        pull.wait()

        if debug:
            print(last_line, end="")

        return r

    def test_wget(self,
                  experiment: ContainerExperiment,
                  dry_run: bool = False,
                  rtt: int = 0,
                  seq: int = 0,
                  r=0,
                  use_old: bool = False,
                  debug=False):
        if r == 0:
            r = random.randrange(999999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("wget", start), end='')
        if not dry_run:
            experiment.add_event("wget%s" % task_suffix, "start", rtt, seq, start, 0)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Pull
        query = ""
        if use_old:
            query = "http://%s/from/_/to/%s" % (
                self.service.config.PROXY_SERVER,
                experiment.get_starlight_image(True)
            )
        else:
            query = "http://%s/from/_/to/%s" % (
                self.service.config.PROXY_SERVER,
                experiment.get_starlight_image(False)
            )
            if experiment.has_old_version():
                query = "http://%s/from/%s/to/%s" % (
                    self.service.config.PROXY_SERVER,
                    experiment.get_starlight_image(True),
                    experiment.get_starlight_image(False)
                )

        cmd = [
            "wget",
            "-O", "%s/test.bin" % self.service.config.TMP,
            "-q", query
        ]

        if debug:
            print(cmd)
        call_wait(cmd, debug)

        ######################################################################
        # Timestamp -------------------------------------------------------------------------
        ts_done = time.time()
        print("%3.6fs" % (ts_done - start))
        if not dry_run:
            experiment.add_event("wget%s" % task_suffix, "done", rtt, seq, ts_done, ts_done - start)
        # -----------------------------------------------------------------------------------
        pass

    ####################################################################################################
    # FS Benchmark
    ####################################################################################################
    def ycsb(self, exp: ContainerExperiment, round, suffix):
        self.ycsb_p = subprocess.Popen(
            ['%s run redis -s -P workloads/tsworkloada -p "redis.host=127.0.0.1" -p "redis.port=6379" '
             '-p measurementtype=timeseries -p timeseries.granularity=10000  -p status.interval=100 '
             '> %s/%s%s-%d-%s.txt' % (
                 self.service.config.YCSB, self.service.config.YCSB_LOG, self.ycsb_base,
                 exp.experiment_name, round, suffix
             )],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            shell=True
        )
        time.sleep(15)
        pass

    def ycsb_terminate(self, debug):
        a, b = self.ycsb_p.communicate()
        if debug:
            print("-------------------------------- YCSB begin")
            if a is not None:
                print(a.decode("utf-8"), end="")
            if b is not None:
                print(b.decode("utf-8"), end="")
            print("-------------------------------- YCSB end", end="")

    ####################################################################################################
    # Memory Testing
    ####################################################################################################
    def get_memory_usage(self, pid, debug):
        if debug:
            print("pid: %d" % pid)

        mem_p = subprocess.Popen("pstree -p -s %d" % pid, shell=True, stdout=subprocess.PIPE,
                                 stderr=subprocess.PIPE)
        a, b = mem_p.communicate()
        a = a.decode("utf-8")
        b = b.decode("utf-8")

        pid_list = ""
        for pid in re.split('\(|\)', a):
            if pid.isdigit():
                if int(pid) > 100:
                    pid_list += pid.strip() + "\\n"

        pid_list = 'echo "' + pid_list + '"'
        if debug:
            print(pid_list)

        mem_p = subprocess.Popen("%s | xargs -I '{}' grep VmHWM /proc/{}/status" % pid_list,
                                 shell=True,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        a, b = mem_p.communicate()
        a = a.decode("utf-8")
        b = b.decode("utf-8")

        if debug:
            print(a, end="")
            print(b, end="")

        mem_usage_list = []
        for mem_usage in a.split(' '):
            if mem_usage.isdigit():
                mem_usage_list.append(int(mem_usage))
        return max(mem_usage_list)


    def save_memory_usage(self,
                          experiment: ContainerExperiment,
                          method: str = "",
                          event: str = "",
                          rtt: int = 0,
                          round: int = 0,
                          other: subprocess.Popen = None,
                          debug: bool = False
                          ):
        try:
            if self.service.p_containerd is not None:
                mem = self.get_memory_usage(self.service.p_containerd.pid, debug=debug)
                experiment.add_event(method, "mem-containerd-%s" % event, rtt, round, delta=mem)
            else:
                if debug:
                    print("get memory containerd not exists")
        except:
            if debug:
                print("get memory error")
            pass

        try:
            if self.service.p_starlight is not None:
                mem = self.get_memory_usage(self.service.p_starlight.pid, debug=debug)
                experiment.add_event(method, "mem-starlight-%s" % event, rtt, round, delta=mem)
            else:
                if debug:
                    print("get memory starlight not exists")
        except:
            if debug:
                print("get memory error")
            pass

        try:
            if self.service.p_stargz is not None:
                mem = self.get_memory_usage(self.service.p_stargz.pid, debug=debug)
                experiment.add_event(method, "mem-stargz-%s" % event, rtt, round, delta=mem)
            else:
                if debug:
                    print("get memory estargz not exists")
        except:
            if debug:
                print("get memory error")
            pass

        try:
            if other is not None:
                mem = self.get_memory_usage(self.service.p_stargz.pid, debug=debug)
                experiment.add_event(method, "mem-%s" % event, rtt, round, delta=mem)
            else:
                if debug:
                    print("get memory other process not exists")
        except:
            if debug:
                print("get memory error")
            pass

    ####################################################################################################
    # Pull and Run
    ####################################################################################################
    def test_estargz(self,
                     experiment: ContainerExperiment,
                     dry_run: bool = False,
                     rtt: int = 0,
                     seq: int = 0,
                     use_old: bool = False,
                     r=0,
                     debug: bool = False,
                     ycsb: bool = False):
        if r == 0:
            r = random.randrange(999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        if ycsb is True:
            print(".", end="")
            self.ycsb(experiment, seq, "estargz%s" % task_suffix)

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("estargz", start), end='')
        if not dry_run:
            experiment.add_event("estargz%s" % task_suffix, "start", rtt, seq, start, 0)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Pull
        cmd_pull = [
            "sudo", "ctr-remote",
            "-n", "xe%d" % r,
            "image", "rpull",
        ]

        if not self.service.config.USE_HTTPS:
            cmd_pull.extend(["--plain-http"])

        cmd_pull.extend(["%s/%s" % (
            self.service.config.REGISTRY_SERVER,
            experiment.get_stargz_image(use_old)
        )])

        if debug:
            print(cmd_pull)

        proc_pull = subprocess.Popen(cmd_pull, preexec_fn=os.setpgrp,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            a, b = proc_pull.communicate(timeout=self.timeout)
            if debug:
                if a is not None:
                    print(a.decode("utf-8"), end="")
                if b is not None:
                    print(b.decode("utf-8"), end="")
        except subprocess.TimeoutExpired:
            experiment.add_event("estargz%s" % task_suffix, "pull-timeout", rtt, seq)
            print('pull-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_pull = time.time()
        print("%3.6fs" % (ts_pull - start), end="\t")
        if not dry_run:
            experiment.add_event("estargz%s" % task_suffix, "pull", rtt, seq, ts_pull, ts_pull - start)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Create
        cmd_create = [
            "sudo", "ctr-remote",
            "-n", "xe%d" % r,
            "c", "create",
            "--snapshotter", "stargz"
        ]

        if experiment.has_mounting is True:
            for m in experiment.mounting:
                m.prepare(r, debug)
                cmd_create.extend(["--mount", m.get_mount_parameter(r)])

        cmd_create.extend(["--env-file", self.service.config.ENV, "--net-host"])

        cmd_create.extend([
            "%s/%s" % (
                self.service.config.REGISTRY_SERVER,
                experiment.get_stargz_image(use_old)
            ),
            "task%d%s" % (r, task_suffix)
        ])

        if experiment.has_args:
            cmd_create.extend(experiment.args)

        if debug:
            print(cmd_create)

        proc_create = subprocess.Popen(cmd_create, preexec_fn=os.setpgrp,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            a, b = proc_create.communicate(timeout=self.timeout)
            if debug:
                if a is not None:
                    print(a.decode("utf-8"), end="")
                if b is not None:
                    print(b.decode("utf-8"), end="")
        except subprocess.TimeoutExpired:
            experiment.add_event("estargz%s" % task_suffix, "create-timeout", rtt, seq)
            print('create-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_create = time.time()
        print("%3.6fs" % (ts_create - start), end="\t")
        if not dry_run:
            experiment.add_event("estargz%s" % task_suffix, "create", rtt, seq, ts_create, ts_create - start)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Task Start
        cmd_start = "sudo ctr -n xe%d t start task%d%s 2>&1 %s" % (
            r, r, task_suffix, self.service.config.TEE_LOG_STARGZ_RUNTIME
        )

        if debug:
            print(cmd_start)

        proc_start = subprocess.Popen(cmd_start,
                                      stdout=subprocess.PIPE,
                                      shell=True)

        last_line = ""
        real_done = False
        for ln in proc_start.stdout:
            line = ln.decode('utf-8')
            last_line = line
            if debug:
                print(line, end='')
            if line.find(experiment.ready_keyword) != -1:
                real_done = True
                break

        ######################################################################
        # Timestamp -------------------------------------------------------------------------
        ts_done = time.time()
        try:
            t_duration = ts_done - start
        except:
            print(last_line, end="")
            if not dry_run:
                experiment.add_event("estargz%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
            return

        if real_done is True:
            print("%3.6fs" % t_duration, end="\t")
            if not dry_run:
                experiment.add_event("estargz%s" % task_suffix, "done", rtt, seq, ts_done, t_duration)
        else:
            print(last_line, end="")
            if not dry_run:
                experiment.add_event("estargz%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
        # -----------------------------------------------------------------------------------

        if ycsb is True:
            self.ycsb_terminate(debug)

        ######################################################################
        # Stop
        time.sleep(1)
        proc_stop = subprocess.Popen("sudo ctr -n xe%d t kill task%d%s 2>&1" % (r, r, task_suffix),
                                     preexec_fn=os.setpgrp,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                     shell=True)
        try:
            proc_stop.wait(timeout=10)
            time.sleep(3)
        except:
            print("[stop-timeout]", end="")
            pass

        try:
            proc_start.wait(timeout=10)
        except:
            print("[proc-timeout]", end="")
            pass

        if debug:
            a, b = proc_stop.communicate()
            if a is not None:
                print(a.decode("utf-8"), end="")
            if b is not None:
                print(b.decode("utf-8"), end="")

        if experiment.has_mounting is True and use_old is False:
            for m in experiment.mounting:
                m.destroy(r, debug)

        # Due to the lazy pulling, we might not have the entire image at this point, but we want to
        # make sure all the layers are ready before proceeding to pulling the updated new image
        if use_old:
            self.service.reset_latency_bandwidth()
            print("deploy", end="")
            complete = 0
            for ln in self.service.p_stargz.stdout:
                line = ln.decode('utf-8')
                if debug:
                    print(line, end="")
                if line.find("resolving") != -1:
                    complete += 1
                if line.find("completed to fetch all layer data in background") != -1:
                    complete -= 1
                    if complete == 0:
                        break
            print("-synced.")
            self.service.set_latency_bandwidth(rtt)
        else:
            print("update")

        self.save_memory_usage(experiment, "estargz%s" % task_suffix, "", rtt, seq, debug=debug)
        return r

    def test_starlight(self,
                       experiment: ContainerExperiment,
                       dry_run: bool = False,
                       rtt: int = 0,
                       seq: int = 0,
                       use_old: bool = False,
                       r=0,
                       debug: bool = False,
                       ycsb: bool = False):
        if r == 0:
            r = random.randrange(999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        if ycsb is True:
            print(".", end="")
            self.ycsb(experiment, seq, "starlight%s" % task_suffix)

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("starlight", start), end='')
        if not dry_run:
            experiment.add_event("starlight%s" % task_suffix, "start", rtt, seq, start, 0)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Pull
        cmd_pull = [
            "sudo", "ctr-starlight",
            "-n", "xs%d" % r,
            "prepare"
        ]
        if use_old:
            cmd_pull.append(experiment.get_starlight_image(old=True))
        else:
            if experiment.has_old_version():
                cmd_pull.append(experiment.get_starlight_image(old=True))
            cmd_pull.append(experiment.get_starlight_image(old=False))

        if debug:
            print(cmd_pull)

        proc_pull = subprocess.Popen(cmd_pull, preexec_fn=os.setpgrp,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            a, b = proc_pull.communicate(timeout=self.timeout)
            if debug:
                if a is not None:
                    print(a.decode("utf-8"), end="")
                if b is not None:
                    print(b.decode("utf-8"), end="")
        except subprocess.TimeoutExpired:
            experiment.add_event("starlight%s" % task_suffix, "pull-timeout", rtt, seq)
            print('pull-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_pull = time.time()
        print("%3.6fs" % (ts_pull - start), end="\t")
        if not dry_run:
            experiment.add_event("starlight%s" % task_suffix, "pull", rtt, seq, ts_pull, ts_pull - start)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Create
        cmd_create = [
            "sudo", "ctr-starlight",
            "--log-level", "debug",
            "-n", "xs%d" % r,
            "create",
        ]

        if experiment.has_mounting is True:
            for m in experiment.mounting:
                m.prepare(r, debug)
                cmd_create.extend(["--mount", m.get_mount_parameter(r)])

        # cmd.extend(["-cp", "%d" % checkpoint])
        cmd_create.extend(["--env-file", self.service.config.ENV])
        cmd_create.extend(["--net-host"])

        if use_old:
            cmd_create.append(experiment.get_starlight_image(old=True))  # Image Combo
            cmd_create.append(experiment.get_starlight_image(old=True))  # Specific Image
        else:
            cmd_create.append(experiment.get_starlight_image(old=False))  # Image Combo
            cmd_create.append(experiment.get_starlight_image(old=False))  # Specific Image

        cmd_create.append("task%d%s" % (r, task_suffix))

        if experiment.has_args:
            cmd_create.extend(experiment.args)

        if debug:
            print(cmd_create)

        proc_create = subprocess.Popen(cmd_create, preexec_fn=os.setpgrp,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            a, b = proc_create.communicate(timeout=self.timeout)
            if debug:
                if a is not None:
                    print(a.decode("utf-8"), end="")
                if b is not None:
                    print(b.decode("utf-8"), end="")
        except subprocess.TimeoutExpired:
            experiment.add_event("estargz%s" % task_suffix, "create-timeout", rtt, seq)
            print('create-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_create = time.time()
        print("%3.6fs" % (ts_create - start), end="\t")
        if not dry_run:
            experiment.add_event("starlight%s" % task_suffix, "create", rtt, seq, ts_create, ts_create - start)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Task Start
        cmd_start = "sudo ctr -n xs%d t start task%d%s 2>&1 %s" % (
            r, r, task_suffix, self.service.config.TEE_LOG_STARLIGHT_RUNTIME
        )

        if debug:
            print(cmd_start)

        proc_start = subprocess.Popen(cmd_start,
                                      stdout=subprocess.PIPE,
                                      shell=True)

        last_line = ""
        real_done = False
        for ln in proc_start.stdout:
            line = ln.decode('utf-8')
            last_line = line
            if debug:
                print(line, end='')
            if line.find(experiment.ready_keyword) != -1:
                real_done = True
                break

        ######################################################################
        # Timestamp -------------------------------------------------------------------------
        ts_done = time.time()
        try:
            t_duration = ts_done - start
        except:
            print(last_line, end="")
            if not dry_run:
                experiment.add_event("starlight%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
            return

        if real_done is True:
            print("%3.6fs" % t_duration, end="\t")
            if not dry_run:
                experiment.add_event("starlight%s" % task_suffix, "done", rtt, seq, ts_done, t_duration)
        else:
            print(last_line, end="")
            if not dry_run:
                experiment.add_event("starlight%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
        # -----------------------------------------------------------------------------------

        if ycsb is True:
            self.ycsb_terminate(debug)
        ######################################################################
        # Stop
        time.sleep(1)
        proc_stop = subprocess.Popen("sudo ctr -n xs%d t kill task%d%s 2>&1" % (r, r, task_suffix),
                                     preexec_fn=os.setpgrp,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                     shell=True)
        try:
            proc_stop.wait(timeout=10)
            time.sleep(3)
        except:
            print("[stop-timeout]", end="")
            pass

        try:
            proc_start.wait(timeout=10)
        except:
            print("[proc-timeout]", end="")
            pass

        if debug:
            a, b = proc_stop.communicate()
            if a is not None:
                print(a.decode("utf-8"), end="")
            if b is not None:
                print(b.decode("utf-8"), end="")

        if experiment.has_mounting is True and use_old is False:
            for m in experiment.mounting:
                m.destroy(r, debug)

        # Due to the lazy pulling, we might not have the entire image at this point, but we want to
        # make sure all the layers are ready before proceeding to pulling the updated new image
        if use_old:
            self.service.reset_latency_bandwidth()
            print("deploy", end="")
            for ln in self.service.p_starlight.stdout:
                line = ln.decode('utf-8')
                if debug:
                    print(line, end="")
                if line.find("entire image extracted") != -1:
                    break
            print("-synced.")
            self.service.set_latency_bandwidth(rtt)
        else:
            print("update")

        self.save_memory_usage(experiment, "starlight%s" % task_suffix, "", rtt, seq, debug=debug)
        return r

    def test_vanilla(self,
                     experiment: ContainerExperiment,
                     dry_run: bool = False,
                     rtt: int = 0,
                     seq: int = 0,
                     use_old: bool = False,
                     r=0,
                     debug: bool = False,
                     ycsb: bool = False):
        if r == 0:
            r = random.randrange(999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        if ycsb is True:
            print(".", end="")
            self.ycsb(experiment, seq, "vanilla%s" % task_suffix)

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("vanilla", start), end='')
        if not dry_run:
            experiment.add_event("vanilla%s" % task_suffix, "start", rtt, seq, start, 0)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Pull
        cmd_pull = [
            "sudo", "ctr",
            "-n", "xv%d" % r,
            "image", "pull"]

        if not self.service.config.USE_HTTPS:
            cmd_pull.extend(["--plain-http"])

        cmd_pull.extend(["%s/%s" % (
            self.service.config.REGISTRY_SERVER,
            experiment.get_vanilla_image(use_old)
        )])

        if debug:
            print(cmd_pull)

        proc_pull = subprocess.Popen(cmd_pull, preexec_fn=os.setpgrp,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            a, b = proc_pull.communicate(timeout=self.timeout)
            if debug:
                if a is not None:
                    print(a.decode("utf-8"), end="")
                if b is not None:
                    print(b.decode("utf-8"), end="")
        except subprocess.TimeoutExpired:
            experiment.add_event("vanilla%s" % task_suffix, "pull-timeout", rtt, seq)
            print('pull-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_pull = time.time()
        print("%3.6fs" % (ts_pull - start), end="\t")
        if not dry_run:
            experiment.add_event("vanilla%s" % task_suffix, "pull", rtt, seq, ts_pull, ts_pull - start)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Create
        cmd_create = [
            "sudo", "ctr",
            "-n", "xv%d" % r,
            "c", "create"
        ]

        if experiment.has_mounting is True:
            for m in experiment.mounting:
                m.prepare(r, debug)
                cmd_create.extend(["--mount", m.get_mount_parameter(r)])

        cmd_create.extend(["--env-file", self.service.config.ENV, "--net-host"])

        cmd_create.extend([
            "%s/%s" % (
                self.service.config.REGISTRY_SERVER,
                experiment.get_vanilla_image(use_old)
            ),
            "task%d%s" % (r, task_suffix)
        ])

        if experiment.has_args:
            cmd_create.extend(experiment.args)

        if debug:
            print(cmd_create)

        proc_create = subprocess.Popen(cmd_create, preexec_fn=os.setpgrp,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            a, b = proc_create.communicate(timeout=self.timeout)
            if debug:
                if a is not None:
                    print(a.decode("utf-8"), end="")
                if b is not None:
                    print(b.decode("utf-8"), end="")
        except subprocess.TimeoutExpired:
            experiment.add_event("estargz%s" % task_suffix, "create-timeout", rtt, seq)
            print('create-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_create = time.time()
        print("%3.6fs" % (ts_create - start), end="\t")
        if not dry_run:
            experiment.add_event("vanilla%s" % task_suffix, "create", rtt, seq, ts_create, ts_create - start)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Task Start
        cmd_start = "sudo ctr -n xv%d t start task%d%s 2>&1 %s" % (
            r, r, task_suffix, self.service.config.TEE_LOG_CONTAINERD_RUNTIME
        )

        if debug:
            print(cmd_start)

        proc_start = subprocess.Popen(cmd_start,
                                      stdout=subprocess.PIPE,
                                      shell=True)
        last_line = ""
        real_done = False
        for ln in proc_start.stdout:
            line = ln.decode('utf-8')
            last_line = line
            if debug:
                print(line, end='')
            if line.find(experiment.ready_keyword) != -1:
                real_done = True
                break

        ######################################################################
        # Timestamp -------------------------------------------------------------------------
        ts_done = time.time()
        try:
            t_duration = ts_done - start
        except:
            print(last_line, end="")
            if not dry_run:
                experiment.add_event("vanilla%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
            return

        if real_done is True:
            print("%3.6fs" % t_duration, end="\t")
            if not dry_run:
                experiment.add_event("vanilla%s" % task_suffix, "done", rtt, seq, ts_done, t_duration)
        else:
            print(last_line, end="")
            if not dry_run:
                experiment.add_event("vanilla%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
        # -----------------------------------------------------------------------------------

        if ycsb is True:
            self.ycsb_terminate(debug)
        ######################################################################
        # Stop
        time.sleep(1)
        proc_stop = start_process_shell(
            "sudo ctr -n xv%d t kill task%d%s 2>&1" % (r, r, task_suffix)
        )
        try:
            proc_stop.wait(timeout=10)
            time.sleep(3)
        except:
            print("[stop-timeout]", end="")
            pass

        try:
            proc_start.wait(timeout=10)
        except:
            print("[proc-timeout]", end="")
            pass

        if debug:
            a, b = proc_stop.communicate()
            if a is not None:
                print(a.decode("utf-8"), end="")
            if b is not None:
                print(b.decode("utf-8"), end="")

        if experiment.has_mounting is True and use_old is False:
            for m in experiment.mounting:
                m.destroy(r, debug)

        if use_old:
            print("deploy")
        else:
            print("update")

        self.save_memory_usage(experiment, "vanilla%s" % task_suffix, "", rtt, seq, debug=debug)
        return r
