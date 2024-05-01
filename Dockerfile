FROM alpine
COPY vf-device-plugin /bin/vf-device-plugin
ENTRYPOINT /bin/vf-device-plugin
