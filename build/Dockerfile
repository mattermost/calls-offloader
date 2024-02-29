# This dockerfile is used to build Mattermost calls-offloader
# A multi stage build, with golang used as a builder
# and gcr.io/distroless/static as runner
ARG GO_VERSION
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} as builder
ARG TARGETOS
ARG TARGETARCH
ARG GO_BUILD_PLATFORMS=${TARGETOS}-${TARGETARCH}

# Setup directories structure and compile
COPY . /src
WORKDIR /src
RUN make go-build

# Shrink final image since we only need the offloader binary
# and use distroless container image as runner for security
FROM --platform=$TARGETPLATFORM gcr.io/distroless/static@sha256:a43abc840a7168c833a8b3e4eae0f715f7532111c9227ba17f49586a63a73848 as runner
ARG TARGETOS
ARG TARGETARCH
COPY --from=builder /src/dist/calls-offloader-${TARGETOS}-${TARGETARCH} /opt/calls-offloader/bin/calls-offloader

# Create and use unprivileged user to run the service
COPY ./build/group ./build/passwd /etc/

USER calls

WORKDIR /opt/calls-offloader/bin
ENTRYPOINT ["./calls-offloader"]
