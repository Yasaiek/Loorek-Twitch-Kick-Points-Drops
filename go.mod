module loorek

go 1.24.1

toolchain go1.24.7

require (
	fyne.io/systray v0.0.0
	github.com/Microsoft/go-winio v0.6.2
	github.com/andybalholm/brotli v1.1.1
	github.com/bogdanfinn/fhttp v0.6.2
	github.com/bogdanfinn/tls-client v1.11.2
	github.com/bogdanfinn/utls v1.7.4-barnius
	golang.org/x/net v0.38.0
	golang.org/x/sys v0.31.0
)

require (
	github.com/bogdanfinn/quic-go-utls v1.0.4-utls // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/tam7t/hpkp v0.0.0-20160821193359-2b70b4024ed5 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)

replace fyne.io/systray => ./third_party/systray

// golang.org/x/* fetched from their GitHub mirrors
replace (
	golang.org/x/crypto => github.com/golang/crypto v0.36.0
	golang.org/x/exp => github.com/golang/exp v0.0.0-20250305212735-054e65f0b394
	golang.org/x/mod => github.com/golang/mod v0.24.0
	golang.org/x/net => github.com/golang/net v0.38.0
	golang.org/x/sync => github.com/golang/sync v0.12.0
	golang.org/x/sys => github.com/golang/sys v0.31.0
	golang.org/x/term => github.com/golang/term v0.30.0
	golang.org/x/text => github.com/golang/text v0.23.0
	golang.org/x/tools => github.com/golang/tools v0.31.0
)
