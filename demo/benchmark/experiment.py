import subprocess, os, signal
import time
import random
import common as c
import numpy as np


####################################################################################################
# Synchronous Pull Image
####################################################################################################

def sync_pull_estargz(cfg, grpc, image_name, r=0, debug=False):
    if r == 0:
        r = random.randrange(999999)

    spe_p = c.start_process_shell([
        "sudo ctr-remote -n xe%d image rpull --plain-http %s:5000/%s-estargz2  2>&1" % (
            r, cfg.REGISTRY_SERVER, image_name)
    ])
    spe_p.wait()

    complete = 0
    for ln in grpc.stdout:
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


def sync_pull_starlight(cfg, grpc, image_name, r=0, debug=False):
    if r == 0:
        r = random.randrange(999999)

    sps_p = c.start_process_shell([
        "sudo ctr-starlight -n xs%d prepare %s-estargz2 2>&1" % (r, image_name)
    ])
    sps_p.wait()

    for ln in grpc.stdout:
        line = ln.decode('utf-8')
        if debug:
            print(line, end="")
        if line.find("entire image extracted") != -1:
            break

    time.sleep(1)
    return r


def sync_pull_vanilla(cfg, image_name, r=0, debug=False):
    if r == 0:
        r = random.randrange(999999)

    pull = c.start_process_shell([
        "sudo ctr -n xv%d image pull --plain-http %s:5000/%s 2>&1" % (r, cfg.REGISTRY_SERVER, image_name)
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


####################################################################################################
# Pull and Run
####################################################################################################
def test_wget(url, history, r=0, debug=False):
    if r == 0:
        r = random.randrange(999999999)

    start = time.time()
    print("%12s : " % "wget", end='')
    ######################################################################
    # Pull
    c.call_wait(["wget", "-O", "/tmp/test.bin?t=%d" % r, "-q", url], debug)

    ######################################################################
    end = time.time()
    dur = end - start
    print("%3.6fs" % dur)
    history.append(dur)
    pass


def test_estargz(cfg, image_name, history, r=0, debug=False):
    if r == 0:
        r = random.randrange(999999999)

    start = time.time()
    print("%12s : " % "estargz", end='')
    ######################################################################
    # Pull
    c.call_wait([
        "sudo", "ctr-remote",
        "-n", "xe%d" % r,
        "image", "rpull",
        "--plain-http", "%s:5000/%s-estargz2" % (cfg.REGISTRY_SERVER, image_name)
    ], debug)

    ######################################################################
    # Create
    c.call_wait([
        "sudo", "ctr-remote",
        "-n", "xe%d" % r,
        "c", "create",
        "--snapshotter", "stargz",
        "--mount","type=bind,src=/tmp/benchmark-folders/m1,dst=/var/lib/mysql,options=rbind:rw",
        "--mount","type=bind,src=/tmp/benchmark-folders/m2,dst=/var/run/mysqld,options=rbind:rw",
        "--env-file", "./all.env",
        "%s:5000/%s-estargz2" % (cfg.REGISTRY_SERVER, image_name),
        "task%d" % r
    ], debug)

    ######################################################################
    # Task Start
    proc = c.start_process_shell(
        "sudo ctr -n xe%d task start task%d 2>&1 | tee -a /tmp/estargz.log" % (r, r)
    )
    last_line = ""
    for ln in proc.stdout:
        line = ln.decode('utf-8')
        last_line = line
        if debug:
            print(line, end='')
        if line.find(cfg.KEYWORD) != -1:
            break

    ######################################################################
    end = time.time()
    try:
        dur = end - start
    except:
        print(last_line, end="")
        history.append(np.nan)
        return

    print("%3.6fs" % dur)
    history.append(dur)

    ######################################################################
    # Stop
    time.sleep(1)
    stop = c.start_process_shell(
        "sudo ctr -n xe%d t kill task%d 2>&1" % (r, r)
    )
    stop.wait()
    proc.wait()

    if debug:
        a, b = stop.communicate()
        print(a.decode("utf-8"), end="")
        print(b.decode("utf-8"), end="")

        a, b = proc.communicate()
        print(a.decode("utf-8"), end="")
        print(b.decode("utf-8"), end="")

    return r


def test_starlight(cfg, image_name, history, r=0, old_image_name="", debug=False, checkpoint=1):
    if r == 0:
        r = random.randrange(999999999)

    start = time.time()
    print("%12s : " % "starlight", end='')
    ######################################################################
    # Pull
    new = image_name + "-estargz2"
    arr = [
        "sudo", "ctr-starlight",
        "-n", "xs%d" % r,
        "prepare", new
    ]
    if old_image_name != "":
        old = old_image_name + "-estargz2"
        arr = [
            "sudo", "ctr-starlight",
            "-n", "xs%d" % r,
            "prepare", old, new
        ]

    c.call_wait(arr, debug)

    ######################################################################
    # Create
    c.call_wait([
        "sudo", "ctr-starlight",
        "-n", "xs%d" % r,
        "create",
        "--mount","type=bind,src=/tmp/benchmark-folders/m1,dst=/var/lib/mysql,options=rbind:rw",
        "--mount","type=bind,src=/tmp/benchmark-folders/m2,dst=/run/mysqld,options=rbind:rw",
        "--env-file", "./all.env",
        new,
        new,
        "task%d" % r,
        "sh",
        "-c",
        "/entrypoint.sh mysqld; ls -alhn /var/lib/; echo ----; ls -alhn /var/lib/mysql"
    ], debug)

    ######################################################################
    # Task Start
    proc = c.start_process_shell(
        "sudo ctr -n xs%d task start task%d 2>&1 | tee -a /tmp/starlight.log" % (r, r)
    )
    last_line = ""
    for ln in proc.stdout:
        line = ln.decode('utf-8')
        last_line = line
        if debug:
            print(line, end='')
        if line.find(cfg.KEYWORD) != -1:
            break

    ######################################################################
    end = time.time()
    try:
        dur = end - start
    except:
        print(last_line, end="")
        history.append(np.nan)
        return

    print("%3.6fs" % dur)
    history.append(dur)

    ######################################################################
    # Stop
    time.sleep(1)
    stop = c.start_process_shell(
        "sudo ctr -n xs%d t kill task%d 2>&1" % (r, r)
    )
    stop.wait()
    proc.wait()

    if debug:
        a, b = stop.communicate()
        print(a.decode("utf-8"), end="")
        print(b.decode("utf-8"), end="")

        a, b = proc.communicate()
        print(a.decode("utf-8"), end="")
        print(b.decode("utf-8"), end="")

    return r


def test_vanilla(cfg, image_name, history, r=0, debug=False):
    if r == 0:
        r = random.randrange(999999999)

    start = time.time()
    print("%12s : " % "vanilla", end='')
    ######################################################################
    # Pull

    pull = c.start_process_shell([
        "sudo ctr -n xv%d image pull --plain-http %s:5000/%s 2>&1" % (r, cfg.REGISTRY_SERVER, image_name)
    ])
    for ln in pull.stdout:
        line = ln.decode('utf-8')
        if debug:
            print(line, end="")
        pass
    pull.wait()

    ######################################################################
    # Create
    c.call_wait([
        "sudo", "ctr",
        "-n", "xv%d" % r,
        "c", "create",
        "--mount","type=bind,src=/tmp/benchmark-folders/m1,dst=/var/lib/mysql,options=rbind:rw",
        "--mount","type=bind,src=/tmp/benchmark-folders/m2,dst=/var/run/mysqld,options=rbind:rw",
        "--env-file", "./all.env",
        "%s:5000/%s" % (cfg.REGISTRY_SERVER, image_name),
        "task%d" % r
    ], debug)

    ######################################################################
    # Task Start
    proc = c.start_process_shell(
        "sudo ctr -n xv%d task start task%d 2>&1 | tee -a /tmp/vanilla.log" % (r, r)
    )
    last_line = ""
    for ln in proc.stdout:
        line = ln.decode('utf-8')
        last_line = line
        if debug:
            print(line, end='')
        if line.find(cfg.KEYWORD) != -1:
            break

    ######################################################################
    end = time.time()
    try:
        dur = end - start
    except:
        print(last_line, end="")
        history.append(np.nan)
        return

    print("%3.6fs" % dur)
    history.append(dur)

    ######################################################################
    # Stop
    time.sleep(1)
    stop = c.start_process_shell(
        "sudo ctr -n xv%d t kill task%d 2>&1" % (r, r)
    )
    stop.wait()
    proc.wait()

    if debug:
        a, b = stop.communicate()
        print(a.decode("utf-8"), end="")
        print(b.decode("utf-8"), end="")

        a, b = proc.communicate()
        print(a.decode("utf-8"), end="")
        print(b.decode("utf-8"), end="")

    return r
