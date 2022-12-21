# syntax=docker/dockerfile:1

# Global ARG. To be used in all stages.
ARG EXPORTER=mq_prometheus

# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
## ### ### ### ### ### ### BUILD ### ### ### ### ### ### ##
# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
FROM golang:1.19 AS builder

ARG EXPORTER

ENV EXPORTER=${EXPORTER} \
    ORG="github.com/ibm-messaging" \
    REPO="mq-metric-samples" \
    RDURL="https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/messaging/mqdev/redist" \
    RDTAR="IBM-MQC-Redist-LinuxX64.tar.gz" \
    VRMF=9.3.1.0 \
    CGO_CFLAGS="-I/opt/mqm/inc/" \
    CGO_LDFLAGS_ALLOW="-Wl,-rpath.*" \
    genmqpkg_incnls=1 \
    genmqpkg_incsdk=1 \
    genmqpkg_inctls=1

# Install packages
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential

# Create directory structure
RUN mkdir -p /go/src /go/bin /go/pkg \
    && chmod -R 777 /go \
    && mkdir -p /go/src/$ORG \
    && mkdir -p /opt/mqm \
    && chmod a+rx /opt/mqm

# Install MQ client
WORKDIR /opt/mqm
RUN curl -LO "$RDURL/$VRMF-$RDTAR" \
    && tar -zxf ./*.tar.gz \
    && rm -f ./*.tar.gz \
    && bin/genmqpkg.sh -b /opt/mqm

# Build Go application
WORKDIR /go/src/$ORG/$REPO
COPY go.mod .
COPY go.sum .
COPY --chmod=777 ./cmd/${EXPORTER} .
COPY vendor ./vendor
COPY pkg ./pkg
RUN go build -mod=vendor -o /go/bin/${EXPORTER} ./*.go

# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
### ### ### ### ### ### ### RUN ### ### ### ### ### ### ###
# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
FROM golang:1.19

ARG EXPORTER

ENV EXPORTER=${EXPORTER} \
    RDURL="https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/messaging/mqdev/redist" \
    RDTAR="IBM-MQC-Redist-LinuxX64.tar.gz" \
    VRMF=9.3.1.0 \
    genmqpkg_incnls=1 \
    genmqpkg_incsdk=1 \
    genmqpkg_inctls=1 \
    LD_LIBRARY_PATH="/opt/mqm/lib64:/usr/lib64" \
    MQ_CONNECT_TYPE=CLIENT \
    IBMMQ_GLOBAL_CONFIGURATIONFILE=/opt/config/${EXPORTER}.yaml

# Create directory structure
RUN mkdir -p /opt/bin \
    && chmod -R 777 /opt/bin \
    && mkdir -p /opt/mqm \
    && chmod a+rx /opt/mqm \
    && mkdir -p /opt/config \
    && chmod a+rx /opt/config

# Install packages
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Install MQ client 
WORKDIR /opt/mqm
RUN curl -LO "$RDURL/$VRMF-$RDTAR" \
    && tar -zxf ./*.tar.gz \
    && rm -f ./*.tar.gz \
    && bin/genmqpkg.sh -b /opt/mqm \
    && mkdir -p /IBM/MQ/data/errors \
    && mkdir -p /.mqm \
    && chmod -R 777 /IBM \
    && chmod -R 777 /.mqm

COPY --chmod=777 --from=builder /go/bin/${EXPORTER} /opt/bin/${EXPORTER}

CMD ["sh", "-c", "/opt/bin/${EXPORTER}"]