import subprocess, os

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
