# Synthetic review fixture only; this is not a recommended deployable image.
FROM openjdk:11.0.15 AS legacy
ENV JAVA_TOOL_OPTIONS="-XX:-UseContainerSupport"
CMD ["sh", "-c", "cat /sys/fs/cgroup/memory/memory.limit_in_bytes"]
