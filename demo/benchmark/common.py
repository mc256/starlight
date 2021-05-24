import os
import os
import random
import re
import subprocess
import time

import numpy as np

from container_experiment import ContainerExperiment
from process_ctrl import start_process_shell, call_wait
from process_service import ProcessService


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

        mem_p = subprocess.Popen("pstree -p %d" % pid, shell=True, stdout=subprocess.PIPE,
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

    def parse_traffic(self, ifconfig: str):
        nrx, ntx = 0, 0
        for li in ifconfig.split('\n'):
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
        proc_stop = subprocess.Popen("sudo ctr -n xe%d t kill -s 9 task%d%s 2>&1" % (r, r, task_suffix),
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
        proc_stop = subprocess.Popen("sudo ctr -n xs%d t kill -s 9 task%d%s 2>&1" % (r, r, task_suffix),
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
            "sudo ctr -n xv%d t kill -s 9 task%d%s 2>&1" % (r, r, task_suffix)
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
