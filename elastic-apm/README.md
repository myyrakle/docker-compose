# Elasticsearch APM

- Elasticsearch, Kibana, APM Server

## How to use

1. `sudo docker compose up elasticsearch kibana`
2. Then, a password and token will be generated in the `config` directory.
3. Go to the Kibana(`http://localhost:15601`) page and log in.
4. On the Kibana page, install the 'elastic apm' and 'fleet server' integrations (for preset configuration)
5. run `sudo docker compose up apm-server` for apm server setting
6. run `sudo docker compose up sample-app`, and `curl http://localhost:8080/health`.
7. Check the trace on the Kibana apm page.
