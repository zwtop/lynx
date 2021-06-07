FROM golang:1.15 as builder

WORKDIR /root/
COPY reporter/ /root/
RUN go mod init github.com/smartxworks/lynx/tests/e2e/tools/e2e-runner
RUN go mod tidy
RUN go build -o /usr/local/bin/e2e-reporter /root/*.go

FROM golang:1.15

COPY hack/ /usr/local/bin/
COPY kube-config /root/.kube/config
COPY id_rsa /root/.ssh/id_rsa
COPY kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/e2e-reporter /usr/local/bin/e2e-reporter

RUN chmod 0755 /usr/local/bin/*
RUN chmod 0600 /root/.ssh/id_rsa

ENV ELF01HOST 192.168.26.105
ENV ELF02HOST 192.168.26.106
ENV ELF03HOST 192.168.26.102

USER root
ENTRYPOINT ["/usr/local/bin/e2e-reporter"]
