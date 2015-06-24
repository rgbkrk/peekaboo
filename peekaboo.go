// peekaboo registers the current machine (or provided IP) to a Rackspace load
// balancer, enabling (or disabling) itself.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/pagination"
	"github.com/rackspace/gophercloud/rackspace"
	"github.com/rackspace/gophercloud/rackspace/lb/v1/nodes"
)

// backoff executes a closure that performs a CLB operation and returns its error condition. If
// a 422 (immutable entity) error is returned, wait interval seconds perturbed by a small random
// amount and try again. If a non-422 error is returned, return that error immediately. If the
// operation is successful, return nil.
func backoff(interval int, action func() error) error {
	err := action()
	for err != nil {
		err = action()

		if casted, ok := err.(*gophercloud.UnexpectedResponseCodeError); ok {
			var waitReason string
			switch casted.Actual {
			case 422:
				waitReason = "Load balancer is immutable."
			case 413:
				waitReason = "Rate limit exceeded."
			default:
				// Non-422 error.
				return err
			}

			// Sleep and retry.
			base := time.Duration(interval) * time.Second
			delta := time.Duration(-1000+rand.Intn(2000)) * time.Millisecond
			d := base + delta

			log.Printf("%s. Sleeping for %s", waitReason, d)
			time.Sleep(d)
		} else {
			// Non-HTTP error
			return err
		}
	}
	return nil
}

// findNodeByIPPort locates a load balancer node by IP and port.
func findNodeByIPPort(
	client *gophercloud.ServiceClient,
	loadBalancerID int,
	address string,
	port int,
) *nodes.Node {

	// nil until found
	var found *nodes.Node

	nodes.List(client, loadBalancerID, nil).EachPage(func(page pagination.Page) (bool, error) {
		lbNodes, err := nodes.ExtractNodes(page)
		if err != nil {
			log.Fatalf("Error while paging load balancer nodes: %v", err)
		}

		for _, trialNode := range lbNodes {
			if trialNode.Address == address && trialNode.Port == port {
				found = &trialNode
				return false, nil
			}

		}

		return true, nil
	})

	return found
}

// getIP attempts to determine which IP to work with.
//
// The IP address is determined by, in order:
// * peekaboo's -ip flag
// * ServiceNet environment variable: $RAX_SERVICENET_IPV4
// * PublicNet environment variable: $RAX_PUBLICNET_IPV4
// * Locating a 10 dot address from the network interfaces, likely ServiceNet.
// * eth0
func getIP(ipPtr *string) (string, error) {
	serviceNetIPv4 := os.Getenv("RAX_SERVICENET_IPV4")
	publicNetIPv4 := os.Getenv("RAX_PUBLICNET_IPV4")

	switch {
	case *ipPtr != "":
		return *ipPtr, nil
	case serviceNetIPv4 != "":
		return serviceNetIPv4, nil
	case publicNetIPv4 != "":
		return publicNetIPv4, nil
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		cidr := addr.String()
		ip := strings.Split(cidr, "/")[0]

		if strings.HasPrefix(ip, "10.") {
			return ip, nil
		}
	}

	// Find eth0
	eth0, err := net.InterfaceByName("eth0")
	if err != nil {
		return "", fmt.Errorf("trouble finding eth0: %v", err)
	}

	addrs, err = eth0.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		cidr := addr.String()
		ip := strings.Split(cidr, "/")[0]

		// Pick out IPv4
		if strings.ContainsRune(ip, '.') {
			return ip, err
		}
	}

	return "", errors.New("no IP found")
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	var err error

	disabledPtr := flag.Bool("disable", false, "Disable the node on the load balancer")
	drainingPtr := flag.Bool("drain", false, "Drain the node from the load balancer")
	deletePtr := flag.Bool("delete", false, "Delete the node from the load balancer")
	ipPtr := flag.String("ip", "", "IP address to register/deregister on the load balancer")
	interval := flag.Int("interval", 5, "Seconds to wait between each modification attempt")

	flag.Parse()

	username := os.Getenv("OS_USERNAME")
	APIKey := os.Getenv("OS_PASSWORD")
	region := os.Getenv("OS_REGION_NAME")

	if username == "" || APIKey == "" || region == "" {
		log.Fatalf("One or more of $OS_USERNAME, $OS_PASSWORD, and $OS_REGION_NAME not set")
	}

	// These get converted into integers later
	strLoadBalancerID := os.Getenv("LOAD_BALANCER_ID")
	strAppPort := os.Getenv("APP_PORT")

	// Retrieve the port for this load balancer node, defaulting to 80.
	var nodePort = 80
	if strAppPort == "" {
		log.Printf("$APP_PORT not set, defaulting to 80")
	} else {
		nodePort, err = strconv.Atoi(strAppPort)
		if err != nil {
			log.Fatalf("Unable to parse integer from $APP_PORT: %v", strAppPort)
		}
	}

	// Determine the IP Address for the load balancer's node
	nodeAddress, err := getIP(ipPtr)
	if err != nil {
		log.Fatalf("Unable to determine IP address: %v", err)
	}

	// Retrieve the Load Balancer ID.
	if strLoadBalancerID == "" {
		log.Fatalln("$LOAD_BALANCER_ID must be set")
	}
	loadBalancerID, err := strconv.Atoi(strLoadBalancerID)
	if err != nil {
		log.Fatalf("$LOAD_BALANCER_ID [%s] is not an integer: %v", strLoadBalancerID, err)
	}

	provider, err := rackspace.AuthenticatedClient(gophercloud.AuthOptions{
		Username: username,
		APIKey:   APIKey,
	})
	if err != nil {
		log.Fatalf("Trouble authenticating to Rackspace: %v", err)
	}

	client, err := rackspace.NewLBV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})
	if err != nil {
		log.Fatalf("Creating load balancer client in %v failed: %v", region, err)
	}

	nodePtr := findNodeByIPPort(client, loadBalancerID, nodeAddress, nodePort)

	if !*deletePtr {
		// Transition an existing node to the desired state. Create a new node if one doesn't exist
		// already.

		condition := nodes.ENABLED
		if *disabledPtr {
			condition = nodes.DISABLED
		} else if *drainingPtr {
			condition = nodes.DRAINING
		}

		log.Printf("Setting %v:%v to be %v on load balancer %v",
			nodeAddress, nodePort, condition, loadBalancerID)

		if nodePtr != nil {
			log.Printf("Updating existing node %v", *nodePtr)

			opts := nodes.UpdateOpts{
				Condition: condition,
			}

			err = backoff(*interval, func() error {
				return nodes.Update(client, loadBalancerID, nodePtr.ID, opts).ExtractErr()
			})
			if err != nil {
				log.Fatalf("Unable to update node: %v", err)
			}

		} else {
			log.Printf("Creating new node.")
			opts := nodes.CreateOpts{
				nodes.CreateOpt{
					Address:   nodeAddress,
					Port:      nodePort,
					Condition: condition,
				},
			}

			var created []nodes.Node
			err = backoff(*interval, func() error {
				created, err = nodes.Create(client, loadBalancerID, opts).ExtractNodes()
				return err
			})
			if err != nil {
				log.Fatalf("Error creating the node: %v", err)
			}
			if len(created) != 1 {
				log.Fatalf("Something went terribly wrong during node creation: %#v", created)
			}
			nodePtr = &created[0]
		}

		// After the update completes, get the final version of the node.
		nodePtr, err := nodes.Get(client, loadBalancerID, nodePtr.ID).Extract()
		if err != nil {
			log.Fatalf("Update to retrieve final node state: %v", err)
		}

		log.Printf("Final node state: %v", *nodePtr)
	} else {
		// Delete an existing node from the balancer. Do nothing if no node exists.
		if nodePtr != nil {
			log.Printf("Deleting existing node %v", *nodePtr)

			err = backoff(*interval, func() error {
				return nodes.Delete(client, loadBalancerID, nodePtr.ID).ExtractErr()
			})
			if err != nil {
				log.Fatalf("Unable to delete node %d: %v", nodePtr.ID, err)
			}

			log.Println("Final node state: Deleted")
		} else {
			log.Println("Node is already gone. Hooray?")
		}
	}

}
