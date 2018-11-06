#!/bin/sh
set -euo pipefail

rm -rf /var/run/docker.*

echo "*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:DOCKER - [0:0]
-A PREROUTING -m addrtype --dst-type LOCAL -j DOCKER
-A OUTPUT ! -d 127.0.0.0/8 -m addrtype --dst-type LOCAL -j DOCKER
-A POSTROUTING -s 172.25.0.0/16 ! -o docker0 -j MASQUERADE
-A DOCKER -i docker0 -j RETURN
COMMIT
*filter
:INPUT ACCEPT [0:0]
:FORWARD ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:DOCKER - [0:0]
:DOCKER-ISOLATION - [0:0]
:DOCKER-USER - [0:0]
-A FORWARD -j DOCKER-USER
-A FORWARD -j DOCKER-ISOLATION
-A FORWARD -o docker0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
-A FORWARD -o docker0 -j DOCKER
-A FORWARD -i docker0 ! -o docker0 -j ACCEPT
-A FORWARD -i docker0 -o docker0 -j ACCEPT
-A DOCKER-ISOLATION -j RETURN
-A DOCKER-USER -s 172.25.0.0/16 -d ${INTERNAL_WEB_IP} -p tcp -m multiport --dports 80,443 -j RETURN
-A DOCKER-USER -s 172.25.0.0/16 -d 172.16.0.0/12 -j DROP
-A DOCKER-USER -s 172.25.0.0/16 -d 192.168.0.0/16 -j DROP
-A DOCKER-USER -s 172.25.0.0/16 -d 10.0.0.0/8 -j DROP
-A DOCKER-USER -s 172.25.0.0/16 -d 100.64.0.0/10 -j DROP
-A DOCKER-USER -j RETURN
COMMIT" | iptables-restore -c

exec dockerd --log-level=error --storage-driver=overlay2 -H unix:///var/run/docker.sock --iptables=0
