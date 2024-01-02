# syntax=docker/dockerfile:1

# This Dockerfile shows how you can both build and run a container with 
# a specific exporter/collector program. It uses two stages, copying the relevant
# material from the build step into the runtime container.
#
# It can cope with both platforms where a Redistributable Client is available, and platforms
# where it is not - copy the .deb install images for such platforms into the MQINST
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
    VRMF=9.3.4.0 \
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
    && mkdir -p /MQINST \
    && chmod a+rx /opt/mqm

# Install MQ client and SDK
# For platforms with a Redistributable client, we can use curl to pull it in and unpack it.
# For most other platforms, we assume that you have deb files available under the current directory
# and we then copy them into the container image. Use dpkg to install from them; these have to be 
# done in the right order. 
#
# The Linux ARM64 image is a full-function server package that is directly unpacked. 
# We only need a subset of the files so strip the unneeded filesets. The download of the image could
# be automated via curl in the same way as the Linux/amd64 download, but it's a much bigger image and
# has a different license. So I'm not going to do that for now.
# 
# If additional Redistributable Client platforms appear, then this block can be altered, including the MQARCH setting.
# 
# The copy of the README is so that at least one file always gets copied, even if you don't have the deb files locally. 
# Using a wildcard in the directory name also helps to ensure that this part of the build always succeeds.
COPY README.md MQINST*/*deb MQINST*/*tar.gz /MQINST

# These are values always set by the "docker build" process
ARG TARGETARCH TARGETOS
RUN echo "Target arch is $TARGETARCH; os is $TARGETOS"
# Might need to refer to TARGET* vars a few times in this block, so define something shorter.
RUN T="$TARGETOS/$TARGETARCH"; \
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
      elif [ "$T" = "linux/arm64" ] ;\
      then \
        cd /MQINST; \
        c=`ls *$VRMF*.tar.gz 2>/dev/null| wc -l`; if [ $c -ne 1 ]; then echo "MQ installation file does not exist in MQINST subdirectory";exit 1;fi; \
        cd /opt/mqm \
        && tar -zxf /MQINST/*.tar.gz \
        && export genmqpkg_incserver=0 \
        && bin/genmqpkg.sh -b /opt/mqm;\
      elif [ "$T" = "linux/ppc64le" -o "$T" = "linux/s390x" ];\
      then \
        cd /MQINST; \
        c=`ls ibmmq-*$VRMF*.deb 2>/dev/null| wc -l`; if [ $c -lt 4 ]; then echo "MQ installation files do not exist in MQINST subdirectory";exit 1;fi; \
        for f in ibmmq-runtime_$VRMF*.deb ibmmq-gskit_$VRMF*.deb ibmmq-client_$VRMF*.deb ibmmq-sdk_$VRMF*.deb; do dpkg -i $f;done; \
      else   \
        echo "Unsupported platform $T";\
        exit 1;\
      fi

# Build the Go application
WORKDIR /go/src/$ORG/$REPO
COPY go.mod .
COPY go.sum .
COPY --chmod=777 ./cmd/${EXPORTER} .
COPY --chmod=777 vendor ./vendor
COPY --chmod=777 pkg ./pkg
# This file holds something like the current commit level if it exists in your tree. It might not be there, so
# we use wildcards and a known file to avoid errors on non-existent files/dirs.
COPY --chmod=777 README.md ./.git*/refs/heads/master* .
RUN buildStamp=`date +%Y%m%d-%H%M%S`; \
    hw=`uname -m`; \
    os=`uname -s`; \
    bp="$os/$hw"; \
    if [ -r master ]; then gitCommit=`cat master`;else gitCommit="Unknown";fi; \
    BUILD_EXTRA_INJECT="-X \"main.BuildStamp=$buildStamp\" -X \"main.BuildPlatform=$bp\" -X \"main.GitCommit=$gitCommit\""; \
    go build -mod=vendor -ldflags "$BUILD_EXTRA_INJECT" -o /go/bin/${EXPORTER} ./*.go

# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
### ### ### ### ### ### ### RUN ### ### ### ### ### ### ###
# --- --- --- --- --- --- --- --- --- --- --- --- --- --- #
FROM golang:1.19 AS runtime

ARG EXPORTER

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

ENV EXPORTER=${EXPORTER} \
    LD_LIBRARY_PATH="/opt/mqm/lib64:/usr/lib64" \
    MQ_CONNECT_TYPE=CLIENT \
    IBMMQ_GLOBAL_CONFIGURATIONFILE=/opt/config/${EXPORTER}.yaml

COPY --chmod=555 --from=builder /go/bin/${EXPORTER} /opt/bin/${EXPORTER}
COPY             --from=builder /opt/mqm/ /opt/mqm/

CMD ["sh", "-c", "/opt/bin/${EXPORTER}"]
