package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/pagination"
	"github.com/rackspace/gophercloud/rackspace"
	"github.com/rackspace/gophercloud/rackspace/lb/v1/lbs"
	"github.com/rackspace/gophercloud/rackspace/lb/v1/nodes"
)

func waitForReady(client *gophercloud.ServiceClient, loadBalancerID int) {

	// Ensure the load balancer is ready
	gophercloud.WaitFor(60, func() (bool, error) {
		lb, err := lbs.Get(client, loadBalancerID).Extract()
		if err != nil {
			return false, err
		}

		if lb.Status != lbs.ACTIVE {
			// It has not yet reached ACTIVE
			return false, nil
		}

		// It has reached ACTIVE
		return true, nil
	})
}

func findNodeByIPPort(
	client *gophercloud.ServiceClient,
	loadBalancerID int,
	address string,
	port int,
) *nodes.Node {

	// nil until found
	var found *nodes.Node

	pager := nodes.List(client, loadBalancerID, nil)

	pager.EachPage(func(page pagination.Page) (bool, error) {
		lbNodes, err := nodes.ExtractNodes(page)
		if err != nil {
			log.Fatalf("Error during paging load balancer: %v", err)
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

func getIP(ipPtr *string) (string, error) {

	// IP overriding order will be:
	//  - The -ip flag takes primary precedence
	//  - Service Net Environment variable
	//  - Public Net Environment variable
	//  - Gleaning a 10 dot out of the network interfaces (typically eth1)
	//  - eth0

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
		return "", fmt.Errorf("Trouble finding eth0: %v", err)
	}

	addrs, err = eth0.Addrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		cidr := addr.String()
		ip := strings.Split(cidr, "/")[0]

		fmt.Println(ip)
	}

	return "", errors.New("Unable to determine an IP for load balancing")

}

func main() {
	var err error

	disabledPtr := flag.Bool("disable", false, "Disable the node on the load balancer")

	//NOTE: peekaboo allows setting the IP by using
	//        - environment variables: RAX_SERVICENET_IPV4 or RAX_PUBLICNET_IPV4
	//        - finding an ip on the system starting with 10. (service net)
	//        - locating the eth0 interface
	//        - providing an ip as a flag is fine too and will take precedence
	ipPtr := flag.String("ip", "", "IP address to register/deregister on the load balancer")
	flag.Parse()

	username := os.Getenv("OS_USERNAME")
	APIKey := os.Getenv("OS_PASSWORD")
	region := os.Getenv("OS_REGION_NAME")

	// These get converted into integers later
	strLoadBalancerID := os.Getenv("LOAD_BALANCER_ID")
	strAppPort := os.Getenv("APP_PORT")

	/**
	 * Retrieve port for load balancer's node, defaulting to 80
	 */
	var nodePort = 80

	if strAppPort == "" {
		log.Printf("$APP_PORT not set, defaulting to 80")
	} else {
		nodePort, err = strconv.Atoi(strAppPort)
		if err != nil {
			log.Fatalf("Unable to parse integer from $APP_PORT: %v\n", strAppPort)
		}
	}

	/**
	 * Determine the IP Address for the load balancer's node
	 */
	nodeAddress, err := getIP(ipPtr)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Determined IP: %v", nodeAddress)
	log.Printf("Determined Port: %v", nodePort)

	/**
	 * Retrive Load Balancer ID
	 */
	loadBalancerID, err := strconv.Atoi(strLoadBalancerID)
	if err != nil {
		log.Fatalf("$LOAD_BALANCER_ID not an integer: %v\n", loadBalancerID)
	}

	provider, err := rackspace.AuthenticatedClient(gophercloud.AuthOptions{
		Username: username,
		APIKey:   APIKey,
	})

	if err != nil {
		log.Fatalf("Trouble authenticating to Rackspace: %v\n", err)
	}

	client, err := rackspace.NewLBV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})

	if err != nil {
		log.Fatalf("Creating load balancer client in %v failed: %v\n", region, err)
	}

	log.Println("Client ready")

	waitForReady(client, loadBalancerID)

	node := findNodeByIPPort(client, loadBalancerID, nodeAddress, nodePort)

	condition := nodes.ENABLED
	if *disabledPtr {
		//TODO: Watch the interface on the right process/container to determine
		//      when connections have dropped, and set to DISABLED
		condition = nodes.DRAINING
	}

	log.Printf("Telling %v on port %v to be %v\n", nodeAddress, nodePort, condition)

	if node != nil {
		log.Printf("Found existing node %v", *node)

		opts := nodes.UpdateOpts{
			Condition: condition,
		}

		updateResult := nodes.Update(client, loadBalancerID, node.ID, opts)
		log.Printf("Update result: %v\n", updateResult)

	} else {
		log.Printf("Creating new node")
		opts := nodes.CreateOpts{
			nodes.CreateOpt{
				Address:   nodeAddress,
				Port:      nodePort,
				Condition: condition,
			},
		}

		nodeList := nodes.Create(client, loadBalancerID, opts)
		log.Printf("Node made, total list: %v\n", nodeList)
	}

}
