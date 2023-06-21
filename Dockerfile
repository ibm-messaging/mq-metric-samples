# syntax=docker/dockerfile:1

# This Dockerfile shows how you can both build and run a container with 
# a specific exporter/collector program. It uses two stages, copying the relevant
# material from the build step into the runtime container.
#
# It can cope with both platforms where a Redistributable Client is available, and platforms
# where it is not - copy the .deb install images for such platforms into the MQDEB
# subdirectory of this repository first.

# Global ARG. To be used in all stages.
# Override with "--build-arg EXPORTER=mq_xxxxx" when building.
ARG EXPORTER=mq_prometheus

# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
## ### ### ### ### ### ### BUILD ### ### ### ### ### ### ##
# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
FROM golang:1.19 AS builder

ARG EXPORTER
ENV EXPORTER=${EXPORTER} \
    ORG="github.com/ibm-messaging" \
    REPO="mq-metric-samples" \
    VRMF=9.3.3.0 \
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
    && mkdir -p /MQDEB \
    && chmod a+rx /opt/mqm

# Install MQ client and SDK
# For platforms with a Redistributable client, we can use curl to pull it in and unpack it.
# For other platforms, we assume that you have the deb files available under the current directory
# and we then copy them into the container image. Use dpkg to install from them; these have to be 
# done in the right order.
# 
# If additional Redistributable Client platforms appear, then this block can be altered, including the MQARCH setting.
# 
# The copy of the README is so that at least one file always gets copied, even if you don't have the deb files locally. 
# Using a wildcard in the directory name also helps to ensure that this part of the build should always succeed.
COPY README.md MQDEB*/*deb /MQDEB

# This is a value always set by the "docker build" process
ARG TARGETPLATFORM
RUN echo "Target arch is $TARGETPLATFORM"
# Might need to refer to TARGETPLATFORM a few times in this block, so define something shorter.
RUN T="$TARGETPLATFORM"; \
      if [ "$T" = "linux/amd64" ]; \
      then \
        MQARCH=X64;\
        RDURL="https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/messaging/mqdev/redist";\
        RDTAR="IBM-MQC-Redist-Linux${MQARCH}.tar.gz"; \
        cd /opt/mqm \
        && curl -LO "$RDURL/$VRMF-$RDTAR" \
        && tar -zxf ./*.tar.gz \
        && rm -f ./*.tar.gz \
        && bin/genmqpkg.sh -b /opt/mqm;\
      elif [ "$T" = "linux/ppc64le" -o "$T" = "linux/s390x" ];\
      then \
        cd /MQDEB; \
        c=`ls ibmmq-*$VRMF*.deb| wc -l`; if [ $c -lt 4 ]; then echo "MQ installation files do not exist in MQDEB subdirectory";exit 1;fi; \
        for f in ibmmq-runtime_$VRMF*.deb ibmmq-gskit_$VRMF*.deb ibmmq-client_$VRMF*.deb ibmmq-sdk_$VRMF*.deb; do dpkg -i $f;done; \
      else   \
        echo "Unsupported platform $T";\
        exit 1;\
      fi

# Build Go application
WORKDIR /go/src/$ORG/$REPO
COPY go.mod .
COPY go.sum .
COPY --chmod=777 ./cmd/${EXPORTER} .
COPY --chmod=777 vendor ./vendor
COPY --chmod=777 pkg ./pkg
RUN go build -mod=vendor -o /go/bin/${EXPORTER} ./*.go

# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
### ### ### ### ### ### ### RUN ### ### ### ### ### ### ###
# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
FROM golang:1.19 AS runtime

ARG EXPORTER
ENV EXPORTER=${EXPORTER} \
    LD_LIBRARY_PATH="/opt/mqm/lib64:/usr/lib64" \
    MQ_CONNECT_TYPE=CLIENT \
    IBMMQ_GLOBAL_CONFIGURATIONFILE=/opt/config/${EXPORTER}.yaml

# Create directory structure
RUN mkdir -p /opt/bin \
    && chmod -R 777 /opt/bin \
    && mkdir -p /opt/mqm \
    && chmod 775 /opt/mqm \
    && mkdir -p /opt/config \
    && chmod a+rx /opt/config

# Install packages
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create MQ client directories
WORKDIR /opt/mqm
RUN mkdir -p /IBM/MQ/data/errors \
    && mkdir -p /.mqm \
    && chmod -R 777 /IBM \
    && chmod -R 777 /.mqm

COPY --chmod=555 --from=builder /go/bin/${EXPORTER} /opt/bin/${EXPORTER}
COPY             --from=builder /opt/mqm/ /opt/mqm/

CMD ["sh", "-c", "/opt/bin/${EXPORTER}"]
