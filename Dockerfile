#==================================================== BUILD STAGE ====================================================#
FROM golang:1.12-alpine AS build

ENV BUILD_UTILITIES="git make"

# Install the build utilities.
RUN apk add --no-cache ${BUILD_UTILITIES}

# Build the software.
COPY . /build
WORKDIR /build
RUN make build
#==================================================== FINAL STAGE ====================================================#
FROM alpine:latest
#==================================================== INFORMATION ====================================================#
LABEL Description="Kubernetes Cloud Controller Manager for Cloud.dk"
LABEL Maintainer="Danitso <info@danitso.com>"
LABEL Vendor="Danitso"
#==================================================== INFORMATION ====================================================#
ENV LANG="C.UTF-8"
ENV REQUIRED_PACKAGES="ca-certificates"

# Install the required packages.
RUN apk add --no-cache ${REQUIRED_PACKAGES}

# Copy the binary from the build stage.
COPY --from=build /build/bin/clouddk-cloud-controller-manager /usr/bin/clouddk-cloud-controller-manager

# Ensure that the binary can be executed.
RUN chmod +x /usr/bin/clouddk-cloud-controller-manager

# Set the entrypoint as we will not be requiring shell access.
ENTRYPOINT [ "/usr/bin/clouddk-cloud-controller-manager" ]
