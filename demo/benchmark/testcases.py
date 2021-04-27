import common as c


class TestMySQL(c.ContainerExperiment):
    def __init__(self, version, old_version):
        super().__init__(
            "mysql",
            "socket: '/var/run/mysqld/mysqld.sock'  port: 3306  MySQL Community Server - GPL",
            version,
            old_version
        )
        self.set_mounting_points([
            c.MountingPoint("/var/lib/mysql", False, "rw", "999:999"),
            c.MountingPoint("/run/mysqld", False, "rw", "999:999")
        ])


class TestMariadb(c.ContainerExperiment):
    def __init__(self, version, old_version):
        super().__init__(
            "mariadb",
            "socket: '/var/run/mysqld/mysqld.sock'  port: 3306  mariadb.org binary distribution",
            version,
            old_version
        )
        self.set_mounting_points([
            c.MountingPoint("/var/lib/mysql", False, "rw", "999:999"),
            c.MountingPoint("/run/mysqld", False, "rw", "999:999")
        ])


class TestRedis(c.ContainerExperiment):
    def __init__(self, version, old_version):
        super().__init__(
            "redis",
            "* Ready to accept connections",
            version,
            old_version
        )
        self.set_mounting_points([
            c.MountingPoint("/data"),
        ])


class TestCassandra(c.ContainerExperiment):
    def __init__(self, version, old_version):
        super().__init__(
            "cassandra",
            "- Startup complete",
            version,
            old_version
        )
