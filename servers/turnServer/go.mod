module turnServer

go 1.24.1

replace (
	mjy/define => ../../define
	mjy/logUtil => ../../logUtil
	mjy/serviceUtil => ../../serviceUtil
)

require (
	github.com/pion/turn/v3 v3.0.3
	golang.org/x/sys v0.28.0
)

require (
	github.com/pion/dtls/v2 v2.2.7 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/stun/v2 v2.0.0 // indirect
	github.com/pion/transport/v2 v2.2.1 // indirect
	github.com/pion/transport/v3 v3.0.2 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	mjy/define v0.0.0 // indirect
	mjy/logUtil v0.0.0-00010101000000-000000000000 // indirect
	mjy/serviceUtil v0.0.0-00010101000000-000000000000 // indirect
)
