## Bullet

As the name, bullet is a simple and fast file sharing tool written in Go.

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

Try sending a file
```console
$ ./bullet send large-video.mp4
Share code: df6YOFss
Sending "large-video.mp4" (104.9MB), waiting for receiver...
Sent 104857600 bytes of data!
$
```

And receiving somewhere else
```console
$ ./bullet/bullet recv df6YOFss
Detected sender's file: "large-video.mp4" (104.9MB)
Received 104857600 bytes of data at "large-video.mp4".
$
```

You can specify the filename for receiving. Use `-o -` to recieve directly to stdout
```console
$ ./bullet recv -o myvideo.mp4 mXmDFGvu
Detected sender's file: "large-video.mp4" (104.9MB)
Received 104857600 bytes of data at "myvideo.mp4".
$
```

Share file with your own share code
```console
$./bullet send -code from-diwas hello.mp4
Share code: from-diwas
Sending "hello.mp4" (104.9MB), waiting for receiver...
Sent 104857600 bytes of data!
$
```

```console
$ ./bullet recv from-diwas
Detected sender's file: "hello.mp4" (104.9MB)
Received 104857600 bytes of data at "hello.mp4".
$
```
