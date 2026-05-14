FROM google/cloud-sdk:slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        google-cloud-cli-gke-gcloud-auth-plugin \
        kubectl \
    && rm -rf /var/lib/apt/lists/*

ENTRYPOINT []
CMD ["kubectl", "version", "--client=true"]
