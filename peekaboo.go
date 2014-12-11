package main

import (
	"flag"
	"log"
	"os"
	"strconv"

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
			log.Panicf("Error during paging load balancer: %v", err)
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

func main() {

	disabledPtr := flag.Bool("disable", false, "Disable the node on the load balancer")
	flag.Parse()

	username := os.Getenv("OS_USERNAME")
	APIKey := os.Getenv("OS_PASSWORD")
	region := os.Getenv("OS_REGION_NAME")

	loadBalancerID, err := strconv.Atoi(os.Getenv("LOAD_BALANCER_ID"))

	//nodeAddress := "10.223.224.23"
	nodeAddress := "10.223.224.24"
	nodePort := 80

	if err != nil {
		log.Panicf("$LOAD_BALANCER_ID not an integer: %v\n", loadBalancerID)
	}

	provider, err := rackspace.AuthenticatedClient(gophercloud.AuthOptions{
		Username: username,
		APIKey:   APIKey,
	})

	if err != nil {
		log.Panicf("%v\n", err)
	}

	client, err := rackspace.NewLBV1(provider, gophercloud.EndpointOpts{
		Region: region,
	})

	if err != nil {
		log.Panicf("%v\n", err)
	}

	log.Println("Client ready")

	waitForReady(client, loadBalancerID)

	node := findNodeByIPPort(client, loadBalancerID, nodeAddress, nodePort)

	condition := nodes.ENABLED
	if *disabledPtr {
		//TODO: Watch the interface on the watched container to determine when connections
		//      have dropped, and set to DISABLED
		condition = nodes.DRAINING
	}

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
