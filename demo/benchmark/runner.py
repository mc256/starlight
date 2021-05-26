import os
import random
import subprocess
import time

import numpy as np

from container_experiment import ContainerExperiment
from process_ctrl import start_process_shell, call_wait
from process_service import ProcessService
from system_info import SystemInfo


class Runner:
    def __init__(self):
        self.service = ProcessService()
        self.system_info = SystemInfo(self.service)
        self.timeout = 120
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
                     debug: bool = False):
        if r == 0:
            r = random.randrange(999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        if experiment.has_workload():
            print(".", end="")
            experiment.workload_p = experiment.workload_fn(seq, "estargz%s" % task_suffix, debug)

        self.system_info.save_traffic_usage(experiment, "estargz%s" % task_suffix, rtt, seq, 'start', debug)

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("estargz", start), end='')
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
        self.system_info.save_traffic_usage(experiment, "estargz%s" % task_suffix, rtt, seq, 'launched', debug)
        try:
            t_duration = ts_done - start
        except:
            print(last_line, end="")
            experiment.add_event("estargz%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
            return

        if real_done is True:
            print("%3.6fs" % t_duration, end="\t")
            experiment.add_event("estargz%s" % task_suffix, "done", rtt, seq, ts_done, t_duration)
        else:
            print(last_line, end="")
            experiment.add_event("estargz%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
        # -----------------------------------------------------------------------------------

        if experiment.has_workload():
            experiment.workload_wait(debug)

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

        self.system_info.save_traffic_usage(experiment, "estargz%s" % task_suffix, rtt, seq, 'done', debug)
        self.system_info.save_memory_usage(experiment, "estargz%s" % task_suffix, rtt, seq, 'done', debug)
        self.system_info.save_cpu_usage(experiment, "estargz%s" % task_suffix, rtt, seq, 'done', debug)
        return r

    def test_starlight(self,
                       experiment: ContainerExperiment,
                       dry_run: bool = False,
                       rtt: int = 0,
                       seq: int = 0,
                       use_old: bool = False,
                       r=0,
                       debug: bool = False):
        if r == 0:
            r = random.randrange(999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        if experiment.has_workload():
            print(".", end="")
            experiment.workload_p = experiment.workload_fn(seq, "starlight%s" % task_suffix, debug)

        self.system_info.save_traffic_usage(experiment, "starlight%s" % task_suffix, rtt, seq, 'start', debug)

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("starlight", start), end='')
        experiment.add_event("starlight%s" % task_suffix, "start", rtt, seq, start, 0)
        # -----------------------------------------------------------------------------------
        ######################################################################
        # Pull
        cmd_pull = [
            "sudo", "ctr-starlight",
            "-n", "xs%d" % r,
            "pull"
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
            experiment.add_event("starlight%s" % task_suffix, "create-timeout", rtt, seq)
            print('create-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_create = time.time()
        print("%3.6fs" % (ts_create - start), end="\t")
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
        self.system_info.save_traffic_usage(experiment, "starlight%s" % task_suffix, rtt, seq, 'launched', debug)
        try:
            t_duration = ts_done - start
        except:
            print(last_line, end="")
            experiment.add_event("starlight%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
            return

        if real_done is True:
            print("%3.6fs" % t_duration, end="\t")
            experiment.add_event("starlight%s" % task_suffix, "done", rtt, seq, ts_done, t_duration)
        else:
            print(last_line, end="")
            experiment.add_event("starlight%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
        # -----------------------------------------------------------------------------------

        if experiment.has_workload():
            experiment.workload_wait(debug)
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

        self.system_info.save_traffic_usage(experiment, "starlight%s" % task_suffix, rtt, seq, 'done', debug)
        self.system_info.save_memory_usage(experiment, "starlight%s" % task_suffix, rtt, seq, 'done', debug)
        self.system_info.save_cpu_usage(experiment, "starlight%s" % task_suffix, rtt, seq, 'done', debug)
        return r

    def test_vanilla(self,
                     experiment: ContainerExperiment,
                     dry_run: bool = False,
                     rtt: int = 0,
                     seq: int = 0,
                     use_old: bool = False,
                     r=0,
                     debug: bool = False):
        if r == 0:
            r = random.randrange(999999)

        task_suffix = "-update"
        if use_old:
            task_suffix = "-scratch"

        if experiment.has_workload():
            print(".", end="")
            experiment.workload_p = experiment.workload_fn(seq, "vanilla%s" % task_suffix, debug)

        self.system_info.save_traffic_usage(experiment, "vanilla%s" % task_suffix, rtt, seq, 'start', debug)

        # Timestamp -------------------------------------------------------------------------
        start = time.time()
        print("%12s : %f \t" % ("vanilla", start), end='')
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
            experiment.add_event("vanilla%s" % task_suffix, "create-timeout", rtt, seq)
            print('create-timeout')
            return

        # Timestamp -------------------------------------------------------------------------
        ts_create = time.time()
        print("%3.6fs" % (ts_create - start), end="\t")
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
            experiment.add_event("vanilla%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
            return

        if real_done is True:
            print("%3.6fs" % t_duration, end="\t")
            experiment.add_event("vanilla%s" % task_suffix, "done", rtt, seq, ts_done, t_duration)
        else:
            print(last_line, end="")
            experiment.add_event("vanilla%s" % task_suffix, "done", rtt, seq, np.nan, np.nan)
        # -----------------------------------------------------------------------------------

        if experiment.has_workload():
            experiment.workload_wait(debug)
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

        self.system_info.save_traffic_usage(experiment, "vanilla%s" % task_suffix, rtt, seq, 'launched', debug)
        self.system_info.save_memory_usage(experiment, "vanilla%s" % task_suffix, rtt, seq, 'done', debug)
        self.system_info.save_cpu_usage(experiment, "vanilla%s" % task_suffix, rtt, seq, 'done', debug)
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
        experiment.add_event("wget%s" % task_suffix, "done", rtt, seq, ts_done, ts_done - start)
        # -----------------------------------------------------------------------------------
        pass
