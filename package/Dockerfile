FROM registry.suse.com/bci/bci-base:16.1

RUN zypper -n in patch && \
    rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*

COPY bin/charts-build-scripts /usr/bin/
CMD ["charts-build-scripts"]
