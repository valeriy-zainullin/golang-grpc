FROM debian:trixie 

RUN \
  apt-get update && \
  apt-get install -y ca-certificates golang && \
  mkdir /main

RUN cat >/entrypoint.bash <<EOF
set -xe

cd /mnt-src
bash build.bash || (rm -f main main-test && exit 1)

mv main /mnt-main
mv main-test /mnt-main-test
EOF

WORKDIR /
ENTRYPOINT [ "bash", "/entrypoint.bash" ]
