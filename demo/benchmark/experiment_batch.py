from billion_bench import BillionBench

if __name__ == '__main__':
    for t in BillionBench:


        print("https://hub.docker.com/_/%s" % t.image_name)
        print("\"%s:%s\"" % (t.image_name, t.version))
        print("\"%s:%s\"" % (t.image_name, t.old_version))

        i = 1
        for m in t.mounting:
            print("mkdir /tmp/t%d"% i)
            i += 1

        i = 1
        for m in t.mounting:
            print("    --mount type=bind,src=/tmp/t%d,dst=%s,options=rbind:rw \\" % (i, m.guest_dst))
            i += 1

        i = 1
        for m in t.mounting:
            print("rm -rf /tmp/t%d"% i)
            i += 1

        print("")
