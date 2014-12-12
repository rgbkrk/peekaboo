peekaboo
========

![Peek a boo my little gopher](https://cloud.githubusercontent.com/assets/836375/5406939/db93466c-8180-11e4-9424-ec85a04db052.gif)

Enables or disables a server on a load balancer on run.

Designed to be bound to another service using systemd.

## Example run

```console
docker run --net=host \
           -e LOAD_BALANCER_ID=8675309 \
           -e OS_USERNAME=rgbkrk \
           -e OS_REGION_NAME=IAD \
           -e OS_PASSWORD=deadbeef13617 \
           rgbkrk/peekaboo
2014/12/12 06:09:53 $APP_PORT not set, defaulting to 80
2014/12/12 06:09:53 Determined IP: 10.184.12.147
2014/12/12 06:09:53 Determined Port: 80
2014/12/12 06:09:53 Client ready
2014/12/12 06:09:55 Telling 10.184.12.147 on port 80 to be ENABLED
2014/12/12 06:09:55 Creating new node
2014/12/12 06:09:58 Updated state for node: {10.184.12.147 256049 80 ONLINE ENABLED 1 PRIMARY}
```

### ROADMAP

* [X] Enable node on load balancer
* [X] Drain node off load balancer conditionally
* [X] Get IP Address from an interface
* [ ] Create basic fleet service file example
* [X] Set up Docker build that builds straight from source
* [ ] Set up Docker build that packages *just* the binary build

### Hacking

Clone this into the right spot on your `$GOPATH` and then

```
go get
go build
```

#### Statically linked Go binary without debugging (dwarf)

Drop the built executable to ~6.1MB from ~8.2MB, courtesy [kelseyhightower](https://github.com/kelseyhightower/contributors):

```bash
CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' .
```

```console
~/go/src/github.com/rgbkrk/peekaboo$ go build
~/go/src/github.com/rgbkrk/peekaboo$ cp peekaboo peekaboo.default
~/go/src/github.com/rgbkrk/peekaboo$ CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' .
~/go/src/github.com/rgbkrk/peekaboo$ ls -lah
total 15M
drwxrwxr-x 3 rgbkrk rgbkrk 4.0K Dec 12 05:30 .
drwxrwxr-x 3 rgbkrk rgbkrk 4.0K Dec 12 05:25 ..
drwxrwxr-x 8 rgbkrk rgbkrk 4.0K Dec 12 05:30 .git
-rw-rw-r-- 1 rgbkrk rgbkrk  276 Dec 12 05:25 .gitignore
-rw-rw-r-- 1 rgbkrk rgbkrk 1.5K Dec 12 05:25 LICENSE
-rwxrwxr-x 1 rgbkrk rgbkrk 6.1M Dec 12 05:27 peekaboo          <--- Cleaned
-rwxrwxr-x 1 rgbkrk rgbkrk 8.2M Dec 12 05:27 peekaboo.default  <--- Original
-rw-rw-r-- 1 rgbkrk rgbkrk 5.8K Dec 12 05:25 peekaboo.go
-rw-rw-r-- 1 rgbkrk rgbkrk 1.6K Dec 12 05:30 README.md
```


