FROM tidepool/tpctl-base:latest
WORKDIR /root/workdir
ENV TERM xterm-256color
ENV TZ America/Los_Angeles
ENV DEBIAN_FRONTEND noninteractive
COPY cmd pkgs lib eksctl /root/tpctl/
RUN cd /root/workdir
CMD [ "/root/tpctl/cmd/tpctl.sh"  ]
