# Build the grafana-dashboard-loader binary
FROM golang:1.14.12 as builder

# Copy in the go src
WORKDIR /go/src/github.com/open-cluster-management/grafana-dashboard-loader

COPY pkg/    pkg/
COPY cmd/main.go ./
COPY go.mod ./

RUN export GO111MODULE=on && go mod tidy

RUN export GO111MODULE=on \
    && CGO_ENABLED=0 GOOS=linux go build -a -o grafana-dashboard-loader main.go \
    && strip grafana-dashboard-loader


FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

WORKDIR /
COPY --from=builder /go/src/github.com/open-cluster-management/grafana-dashboard-loader/grafana-dashboard-loader .

ENTRYPOINT ["/grafana-dashboard-loader"]
