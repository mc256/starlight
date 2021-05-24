import random

from process_ctrl import start_process


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
