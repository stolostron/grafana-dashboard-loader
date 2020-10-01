# grafana-dashboard-loader

The grafana-dashboard-loader is a small HTTP reverse proxy.

## Usage

```
$ git clone git@github.com:open-cluster-management/grafana-dashboard-loader.git
$ cd grafana-dashboard-loader
$ go build && ./grafana-dashboard-loader --metrics-server="http://localhost:9090/"
I0723 15:40:18.460797   21486 main.go:48] Proxy server will running on: :3002
I0723 15:40:18.460913   21486 main.go:49] Metrics server is: http://localhost:9090/
```