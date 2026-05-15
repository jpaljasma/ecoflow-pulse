FROM google/cloud-sdk:slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        google-cloud-cli-gke-gcloud-auth-plugin \
        kubectl \
        netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY scripts/cloud-forward-probe.sh scripts/cloud-forward-supervisor.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/cloud-forward-probe.sh /usr/local/bin/cloud-forward-supervisor.sh

ENTRYPOINT []
CMD ["kubectl", "version", "--client=true"]
