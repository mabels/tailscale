docker manifest create fastandfearless/tailscale:latest \
	fastandfearless/tailscale:latest-amd64 \
	fastandfearless/tailscale:latest-arm64v8 \
	fastandfearless/tailscale:latest-arm32v7
docker manifest push --purge fastandfearless/tailscale:latest
