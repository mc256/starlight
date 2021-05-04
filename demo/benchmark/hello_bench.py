"""
The MIT License (MIT)

Copyright (c) 2015 Tintri

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
"""

# Number of download collected from https://hub.docker.com/ on May 3rd, 2021

HelloBenchList = [
    ('alpine', 'distro', '1B'),
    ('busybox', 'distro', '1B'),
    ('crux', 'distro', '500K'),
    ('cirros', 'distro', '5M'),
    ('debian', 'distro', '500M'),
    ('ubuntu', 'distro', '1B'),
    ('ubuntu-upstart', 'distro', '1M'),
    ('ubuntu-debootstrap', 'distro', '5M'),
    ('centos', 'distro', '500M'),
    ('fedora', 'distro', '50M'),
    ('opensuse', 'distro', '5M'),
    ('oraclelinux', 'distro', '10M'),
    ('mageia', 'distro', '1M'),

    ('mysql', 'database', '1B'),
    ('percona', 'database', '50M'),
    ('mariadb', 'database', '1B'),
    ('postgres', 'database', '1B'),
    ('redis', 'database', '1B'),
    ('crate', 'database', '10M'),
    ('rethinkdb', 'database', '50M'),

    ('php', 'language', '500M'),
    ('ruby', 'language', '500M'),
    ('jruby', 'language', '10M'),
    ('julia', 'language', '5M'),
    ('perl', 'language', '100M'),
    ('rakudo-star', 'language', '1M'),
    ('pypy', 'language', '10M'),
    ('python', 'language', '1B'),
    ('golang', 'language', '1B'),
    ('clojure', 'language', '10M'),
    ('haskell', 'language', '5M'),
    ('hylang', 'language', '10M'),
    ('java', 'language', '1B'),
    ('mono', 'language', '10M'),
    ('r-base', 'language', '5M'),
    ('gcc', 'language', '10M'),
    ('thrift', 'language', '1M'),

    ('cassandra', 'database', '100M'),
    ('mongo', 'database', '1B'),
    ('elasticsearch', 'database', '500M'),

    ('hello-world', 'application', '1B'),
    ('ghost', 'application', '100M'),
    ('drupal', 'application', '100M'),
    ('jenkins', 'application', '100M'),
    ('sonarqube', 'application', '100M'),
    ('rabbitmq', 'application', '1B'),
    ('registry', 'application', '1B'),

    ('httpd', 'web-server', '1B'),
    ('nginx', 'web-server', '1B'),
    ('glassfish', 'web-server', '1M'),
    ('jetty', 'web-server', '10M'),
    ('php-zendserver', 'web-server', '1M'),
    ('tomcat', 'web-server', '100M'),

    ('django', 'web-framework', '10M'),
    ('rails', 'web-framework', '5M'),
    ('node', 'web-framework', '1B'),
    ('iojs', 'web-framework', '10M'),
]
