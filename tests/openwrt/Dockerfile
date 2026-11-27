ARG OPENWRT_IMAGE
# Release tags are pinned in docker-compose.yml. The package is installed the way
# the Setup guide installs it, at the feed's current version, as customers get it.
# hadolint ignore=DL3006
FROM ${OPENWRT_IMAGE}
# hadolint ignore=DL3018,DL3019
RUN mkdir -p /var/lock && \
    if command -v apk >/dev/null; then apk update && apk add https-dns-proxy; \
    else opkg update && opkg install https-dns-proxy; fi
COPY entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
