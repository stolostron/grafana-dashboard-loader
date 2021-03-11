module github.com/open-cluster-management/grafana-dashboard-loader

go 1.14

require (
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/klog v1.0.0
)

// Resolves CVE-2020-14040
replace golang.org/x/text => golang.org/x/text v0.3.5