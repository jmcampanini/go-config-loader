module github.com/jmcampanini/go-config-loader/examples/slices

go 1.22

require (
	github.com/jmcampanini/go-config-loader v0.0.0
	github.com/spf13/pflag v1.0.10
)

require github.com/BurntSushi/toml v1.6.0 // indirect

replace github.com/jmcampanini/go-config-loader => ../..
