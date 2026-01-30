cd servers\mediaServer
go clean
go build -ldflags "-s -w"
cd ..\..

cd servers\turnServer
go clean
go build -ldflags "-s -w"
cd ..\..
