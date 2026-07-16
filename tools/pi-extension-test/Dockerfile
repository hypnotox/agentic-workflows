FROM node:22.22.0-alpine@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34
WORKDIR /opt/awf-pi-test
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts && mkdir -p /workspace/node_modules /workspace/repo
COPY docker-entrypoint.sh ./
ENTRYPOINT ["/opt/awf-pi-test/docker-entrypoint.sh"]
