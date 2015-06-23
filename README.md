peekaboo
========

Rackspace load balancing toggler for your host's services.

![Peek a boo my little gopher](https://cloud.githubusercontent.com/assets/836375/5406939/db93466c-8180-11e4-9424-ec85a04db052.gif)

Goal: Automagically connect and disconnect applications to Rackspace load balancers as they come online or are taken offline.

## Example run

Start with a fresh host with Docker (or use a built binary):

```console
$ docker run --net=host \
           -e LOAD_BALANCER_ID=8675309 \
           -e OS_USERNAME=rgbkrk \
           -e OS_REGION_NAME=IAD \
           -e OS_PASSWORD=deadbeef13617 \
           rgbkrk/peekaboo
2014/12/12 06:36:49 $APP_PORT not set, defaulting to 80
2014/12/12 06:36:51 Setting 10.184.12.147:80 to be ENABLED on load balancer 64965
2014/12/12 06:36:51 Creating new node
2014/12/12 06:36:53 Final node state: {10.184.12.147 256073 80 ONLINE ENABLED 1 PRIMARY}
```

Awesome, this host is now connected to the load balancer. Now drain connections from it!

```console
$ docker run --net=host \
           -e LOAD_BALANCER_ID=8675309 \
           -e OS_USERNAME=rgbkrk \
           -e OS_REGION_NAME=IAD \
           -e OS_PASSWORD=deadbeef13617 \
           rgbkrk/peekaboo -drain
2014/12/12 06:37:01 $APP_PORT not set, defaulting to 80
2014/12/12 06:37:02 Setting 10.184.12.147:80 to be DRAINING on load balancer 64965
2014/12/12 06:37:02 Updating existing node {10.184.12.147 256073 80 ONLINE ENABLED 0 PRIMARY}
2014/12/12 06:37:06 Final node state: {10.184.12.147 256073 80 ONLINE DRAINING 1 PRIMARY}
```

Back online for good measure:

```console
$ docker run --net=host \
           -e LOAD_BALANCER_ID=8675309 \
           -e OS_USERNAME=rgbkrk \
           -e OS_REGION_NAME=IAD \
           -e OS_PASSWORD=deadbeef13617 \
           rgbkrk/peekaboo
2014/12/12 06:37:18 $APP_PORT not set, defaulting to 80
2014/12/12 06:37:20 Setting 10.184.12.147:80 to be ENABLED on load balancer 64965
2014/12/12 06:37:20 Updating existing node {10.184.12.147 256073 80 ONLINE DRAINING 0 PRIMARY}
2014/12/12 06:37:23 Final node state: {10.184.12.147 256073 80 ONLINE ENABLED 1 PRIMARY}
```

Now kill it entirely:

```console
$ docker run --net=host \
           -e LOAD_BALANCER_ID=8675309 \
           -e OS_USERNAME=rgbkrk \
           -e OS_REGION_NAME=IAD \
           -e OS_PASSWORD=deadbeaf13617 \
           rgbkrk/peekaboo -delete
2015/06/23 13:03:48 Deleting existing node {10.209.136.41 938559 80 ONLINE DRAINING 0 PRIMARY}
2015/06/23 13:03:48 Final node state: Deleted
```

### Configuration

#### Load Balancer ID

Set environment variable `LOAD_BALANCER_ID` to the ID of the Load Balancer you wish to connect servers to. The load balancer must exist ahead of time.

The ID can be found on the details page for the load balancer in your Rackspace control panel.

#### Rackspace Credentials and Region

We use the standard OpenStack environment variable names here:

`OS_USERNAME` - User name to log in to the Rackspace control panel
`OS_PASSWORD` - [API Key for Rackspace](http://www.rackspace.com/knowledge_center/article/view-and-reset-your-api-key)
`OS_REGION_NAME` - Any one of IAD, DFW, HKG, SYD, ORD (haven't tested LON)

#### IP Address

Peekaboo will try to determine what IP to use with the load balancer. It does this by looking for a 10.x.x.x IPv4 address on the host then by looking for an `eth0` interface.

If you need control you can set the `-ip` flag (e.g. `-ip 192.168.1.3` or define one of two environment variables (adopted from [coreos-cluster](https://github.com/kenperkins/coreos-cluster)): `RAX_SERVICENET_IPV4` or `RAX_PUBLICNET_IPV4`.

Precedence order:

* `-ip` flag
* `$RAX_SERVICENET_IPV4`
* `$RAX_PUBLICNET_IPV4`
* Gleaning a 10 dot out of the network interfaces (*most likely*)
* IPv4 address from `eth0` (last ditch option)

#### Application port

Set `$APP_PORT` or just let 80 be the default. Your choice.

### ROADMAP

* [X] Enable node on load balancer
* [X] Drain node off load balancer conditionally
* [X] Get IP Address from an interface
* [ ] Create basic fleet service file example
* [X] Set up Docker build that builds straight from source
* [ ] Set up Docker build that packages *just* the binary build
* [ ] Determine when the node has terminated connections after DRAINING, set to DISABLED and allow follow on processes to continue (CoreOS update, fleet movement)

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
