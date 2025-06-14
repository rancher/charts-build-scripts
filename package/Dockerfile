FROM registry.suse.com/bci/bci-base:15.6

RUN zypper -n in patch && \
    rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*

COPY bin/charts-build-scripts /usr/bin/
CMD ["charts-build-scripts"]
