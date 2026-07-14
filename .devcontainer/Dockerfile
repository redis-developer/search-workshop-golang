FROM mcr.microsoft.com/devcontainers/go:1.25-bookworm

# ONNX Runtime shared library for the local embedding model (hf module).
# Pinned so every participant runs the same inference stack.
ARG ONNXRUNTIME_VERSION=1.22.0
RUN ARCH=$(dpkg --print-architecture) \
    && case "$ARCH" in \
         amd64) ORT_ARCH=x64 ;; \
         arm64) ORT_ARCH=aarch64 ;; \
         *) echo "unsupported arch: $ARCH" && exit 1 ;; \
       esac \
    && curl -fsSL -o /tmp/ort.tgz \
       "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}.tgz" \
    && tar -xzf /tmp/ort.tgz -C /tmp \
    && cp "/tmp/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION}" /usr/local/lib/libonnxruntime.so \
    && rm -rf /tmp/ort.tgz "/tmp/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}" \
    && ldconfig

# uv for the Lab 8 Python optimizer project.
RUN curl -fsSL https://astral.sh/uv/install.sh | env UV_INSTALL_DIR=/usr/local/bin sh
