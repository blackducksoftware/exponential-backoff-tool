module github.com/blackducksoftware/exponential-backoff-tool

go 1.23.6

// toolchain pins the minimum Go release that must compile this module.
// go1.27.3 is the first 1.27.x release that distrusts compromised eMudhra
// intermediates (CVE-2026-111943). Binaries built with older toolchains are
// flagged by scan-certs.
toolchain go1.27.3

require (
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/mattn/go-shellwords v1.0.17
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/spf13/cobra v1.10.2
	gopkg.in/ini.v1 v1.68.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)
