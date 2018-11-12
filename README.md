# swarm-exporter

A prometheus export which exports several metrics about a docker swarm cluster, e.g. node availability, number of running replicas of a service etc.

## Running

The exporter will need acccess to the docker API, it needs to run on a swarm manager node:
```
docker run -d --name swarm-exporter -v /var/run/docker.sock:/var/run/docker.sock -p 9515:9515 jeinwag/swarm-exporter
``` 

## Building

Just run ``docker build .``.


