# See here for image contents: https://github.com/microsoft/vscode-dev-containers/tree/v0.166.0/containers/go/.devcontainer/base.Dockerfile

# [Choice] Go version: 1, 1.16, 1.15
ARG VARIANT="1"
FROM mcr.microsoft.com/vscode/devcontainers/go:0-${VARIANT}

# [Option] Install Node.js
ARG INSTALL_NODE="true"
ARG NODE_VERSION="lts/*"
RUN if [ "${INSTALL_NODE}" = "true" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

# build dependencies
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
        libx11-dev libxrandr-dev libxtst-dev libgtk-3-dev \
        libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev

# runtime dependencies
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
        wget ca-certificates supervisor \
        pulseaudio dbus-x11 xserver-xorg-video-dummy xserver-xorg-input-void \
        libcairo2 libxcb1 libxrandr2 libxv1 libopus0 libvpx5 \
        #
        # file chooser handler, clipboard
        xdotool xclip \
        #
        # gst
        gstreamer1.0-plugins-base gstreamer1.0-plugins-good \
        gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly \
        gstreamer1.0-pulseaudio;

# browser dependencies
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends openbox chromium;

# configure runtime
ARG USERNAME=neko
ARG USER_UID=1001
ARG USER_GID=$USER_UID
RUN set -eux; \
    #
    # create a non-root user
    groupadd --gid $USER_GID $USERNAME; \
    useradd --uid $USER_UID --gid $USERNAME --shell /bin/bash --create-home $USERNAME; \
    adduser $USERNAME audio; \
    adduser $USERNAME video; \
    adduser $USERNAME pulse; \
    #
    # add sudo support
    apt-get update; \
    apt-get install -y sudo; \
    echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME; \
    chmod 0440 /etc/sudoers.d/$USERNAME; \
    #
    # setup pulseaudio
    mkdir -p /home/$USERNAME/.config/pulse/; \
    echo "default-server=unix:/tmp/pulseaudio.socket" > /home/$USERNAME/.config/pulse/client.conf; \
    #
    # workaround for an X11 problem: http://blog.tigerteufel.de/?p=476
    mkdir /tmp/.X11-unix; \
    chmod 1777 /tmp/.X11-unix; \
    chown $USERNAME /tmp/.X11-unix/; \
    #
    # make directories for neko
    mkdir -p /etc/neko /var/www /var/log/neko; \
    chmod 1777 /var/log/neko; \
    chown $USERNAME /var/log/neko/; \
    chown -R $USERNAME:$USERNAME /home/$USERNAME

# copy runtime files
COPY runtime/dbus /usr/bin/dbus
COPY runtime/default.pa /etc/pulse/default.pa
COPY runtime/supervisord.conf /etc/neko/supervisord.conf
COPY runtime/xorg.conf /etc/neko/xorg.conf
COPY runtime/icon-theme /home/$USERNAME/.icons/default

# copy browser files
COPY runtime/browser/env_wrapper /usr/bin/env_wrapper
# COPY runtime/browser/openbox.xml /etc/neko/openbox.xml
COPY runtime/browser/supervisord.chromium.conf /etc/neko/supervisord/chromium.conf
COPY runtime/browser/extension /usr/share/chromium/extensions/neko
# COPY runtime/browser/policies.json /etc/chromium/policies/managed/policies.json
COPY --chown=$USERNAME runtime/browser/preferences.json /home/neko/.config/chromium/Default/Preferences

# copy dev files
COPY dev/runtime/openbox.xml /etc/neko/openbox.xml
COPY dev/runtime/policies.json /etc/chromium/policies/managed/policies.json
COPY dev/runtime/config.yml /etc/neko/neko.yml
COPY dev/runtime/supervisord.debug.conf /etc/neko/supervisord/debug.conf

# customized scripts
RUN chmod +x /usr/bin/dbus;\
    echo '#!/bin/sh\nsleep infinity' > /usr/bin/neko; \
    chmod +x /usr/bin/neko; \
    echo '#!/bin/sh\nsudo sh -c "export USER='$USERNAME'\nexport HOME=/home/'$USERNAME'\n/usr/bin/supervisord -c /etc/neko/supervisord.conf"' > /usr/bin/deps; \
    chmod +x /usr/bin/deps;

# set default envs
ENV USER=$USERNAME
ENV DISPLAY=:99.0
ENV NEKO_SERVER_BIND=:3000
ENV NEKO_WEBRTC_EPR=3001-3004
ENV BROWSER_KIOSK=false