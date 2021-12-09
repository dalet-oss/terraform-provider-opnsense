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

func create(dhcp *opnsense.DHCPStaticLeases, lease *opnsense.DHCPLease) {
	err := dhcp.CreateLease(lease)
	if err != nil {
		panic(err)
	}
}

func edit(dhcp *opnsense.DHCPStaticLeases, orig, new *opnsense.DHCPLease) {
	err := dhcp.EditLease(orig, new)
	if err != nil {
		panic(err)
	}
}

func compare(dhcp *opnsense.DHCPStaticLeases, count, expect int, verbose bool) {
	fmt.Printf("[R] Reading out DHCP leases ...\n")
	read(dhcp, verbose)
	c := len(dhcp.Leases)
	fmt.Printf("[R] Found %d DHCP leases.\n", c)
	if c == expect {
		fmt.Printf("[R] We're good to go !\n")
	} else {
		fmt.Printf("[R] Something went wrong ;-(\n")
	}
}

func dhcpTest(dhcp *opnsense.DHCPStaticLeases, verbose bool) {
	origLease := opnsense.DHCPLease{
		Interface: "opt3",
		IP:        "10.69.0.99",
		MAC:       "aa:bb:cc:dd:ee:ff",
		Hostname:  "terraform",
	}

	newLease := opnsense.DHCPLease{
		Interface: "opt3",
		IP:        "10.69.0.100",
		MAC:       "aa:bb:cc:dd:ee:ff",
		Hostname:  "terraform2",
	}

	fmt.Printf("[R] Reading out DHCP leases ...\n")
	read(dhcp, verbose)
	count := len(dhcp.Leases)
	fmt.Printf("[R] Found %d DHCP leases.\n", count)

	fmt.Printf("[C] Creating new static DHCP lease ...\n")
	create(dhcp, &origLease)
	compare(dhcp, count, count+1, verbose)

	fmt.Printf("[U] Updating existing static DHCP lease ...\n")
	edit(dhcp, &origLease, &newLease)
	compare(dhcp, count, count+1, verbose)

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
