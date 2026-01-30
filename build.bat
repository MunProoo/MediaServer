cd servers\mediaServer
go clean
go build -o ..\..\mediaServer.exe -ldflags "-s -w" .
cd ..\..

cd servers\turnServer
go clean
go build -o ..\..\turnServer.exe -ldflags "-s -w" .
cd ..\..
