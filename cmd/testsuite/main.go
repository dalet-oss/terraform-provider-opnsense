package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gxben/terraform-provider-opnsense/opnsense"
)

func read(dhcp *opnsense.DHCPStaticLeases, verbose bool) {
	err := dhcp.GetLeases()
	if err != nil {
		panic(err)
	}

	if verbose {
		dhcp.PrintLeases()
	}
}

func create(dhcp *opnsense.DHCPStaticLeases) {
	lease := opnsense.DHCPLease{
		Interface: "opt3",
		IP:        "10.69.0.99",
		MAC:       "aa:bb:cc:dd:ee:ff",
		Hostname:  "terraform",
	}

	err := dhcp.CreateLease(&lease)
	if err != nil {
		panic(err)
	}
}

func dhcpTest(dhcp *opnsense.DHCPStaticLeases, verbose bool) {
	fmt.Printf("[R] Reading out DHCP leases ...\n")
	read(dhcp, verbose)
	count := len(dhcp.Leases)
	fmt.Printf("[R] Found %d DHCP leases.\n", count)
	fmt.Printf("[C] Creating new static DHCP lease ...\n")
	create(dhcp)
	fmt.Printf("[R] Reading out DHCP leases ...\n")
	read(dhcp, verbose)
	c := len(dhcp.Leases)
	fmt.Printf("[R] Found %d DHCP leases.\n", c)
	if c == (count + 1) {
		fmt.Printf("[R] That's one more, we're good to go !\n")
	} else {
		fmt.Printf("[R] Something went wrong ;-(\n")
	}
}

func main() {
	uriFlag := flag.String("uri", "", "The OPNSense root uri to connect to")
	userFlag := flag.String("user", "", "The OPNSense user ID to connect with")
	passwordFlag := flag.String("password", "", "The OPNSense user password to connect with")
	verboseFlag := flag.Bool("verbose", false, "add extra debug info")
	flag.Parse()

	if *uriFlag == "" || *userFlag == "" || *passwordFlag == "" {
		fmt.Printf("Missing arguments. Bailing out ...\n")
		os.Exit(0)
	}

	var dhcp = opnsense.DHCPStaticLeases{}

	err := dhcp.Authenticate(*uriFlag, *userFlag, *passwordFlag)
	if err != nil {
		panic(err)
	}

	// run CRUD operations test
	dhcpTest(&dhcp, *verboseFlag)
}
