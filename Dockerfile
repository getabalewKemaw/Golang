# ---------------------------------------------------------------------
# Production Minimal Runner Image (~15MB)
# Runs the standalone statically compiled Linux binary
# ---------------------------------------------------------------------
FROM alpine:3.20 AS runner

# Install CA certificates for TLS/HTTPS and IANA timezone data
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the statically compiled standalone Linux binary
COPY bin/task-manager-api /bin/task-manager-api

# Ensure executable permissions
RUN chmod +x /bin/task-manager-api

# Run as unprivileged non-root user for security
USER appuser

EXPOSE 8080

ENTRYPOINT ["/bin/task-manager-api"]
