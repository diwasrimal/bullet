## Bullet

As the name, bullet is a simple and fast peer to peer file sharing tool written in Go.

### Build
```sh
git clone github.com/diwasrimal/bullet.git
cd bullet
go build ./cmd/bullet-server
go build ./cmd/bullet
```

### Run

Run the server
```sh
./bullet-server
```

Try send a file from one connection
```sh
$ ./bullet send large-video.webm
Use id MjeNToVd to share the file
Sent 849814137 bytes of data
$
```

And receive from other
```sh
$ ./bullet recv MjeNToVd > received.webm
$
```
