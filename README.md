# Cloudflare Certs Renewer

`cfcr` is an utility that updates TXT records when needed in order for Cloudflare Advanced Edge Certificates to be renewed. When using Let's Encrypt provider, this operation must be done manually every 3 months, which end up being time-consuming.

## Usage

The CLI usage of `cfcr` is really simple, as everything is configured using a YAML file ([see](#config)):

```
Usage of cfcr:
  -config string
        configuration file name (default "config.yaml")
  -run-once bool
        run the program once and do not loop forever
```
## Config

Most of `cfcr` configuration is done using a YAML config file. A sample is provided [here](https://github.com/govirtuo/cfcr/blob/main/config.sample.yaml). For now, only one DNS provider is supported: OVH. If you need another one, feel free to contribute! The integration if new providers should be easy thanks to the `Providers` interface.

## Metrics

`cfcr` is shipped with an embedded Prometheus exporter that exposes basic metrics about the program behavior (stack/heap allocations...) and some others about certs renewal, especially:

* `cfcr_domains_watched_total`: the number of domains `cfcr` is watching;
* `cfcr_last_updated_timestamp`: when was a given domain last updated by `cfcr`. There is one version of this metric for each watched domain.

## Internals

![Workflow](docs/certs-check-diagram.png)

## License

[MIT](LICENSE)