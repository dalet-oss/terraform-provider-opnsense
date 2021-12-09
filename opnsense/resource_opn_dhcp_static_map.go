package opnsense

import (
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

const (
	// KeyInterface corresponds to the associated resource schema key
	KeyInterface = "interface"
	// KeyMAC corresponds to the associated resource schema key
	KeyMAC = "mac"
	// KeyIP corresponds to the associated resource schema key
	KeyIP = "ipaddr"
	// KeyName corresponds to the associated resource schema key
	KeyName = "hostname"
)

func resourceOpnDHCPStaticMap() *schema.Resource {
	return &schema.Resource{
		Create: resourceDhcpStaticMappingCreate,
		Read:   resourceDhcpStaticMappingRead,
		Update: resourceDhcpStaticMappingUpdate,
		Delete: resourceDhcpStaticMappingDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			KeyInterface: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyMAC: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsMACAddress,
			},
			KeyIP: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsIPAddress,
			},
			KeyName: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
		},
	}
}

var rxRsID = regexp.MustCompile("([^/]+)/([^/]+)")

func parseDhcpResourceID(resID string) (itf string, mac string, err error) {
	if !rxRsID.MatchString(resID) {
		return "", "", fmt.Errorf("invalid resource format: %s. must be interface/mac", resID)
	}
	idMatch := rxRsID.FindStringSubmatch(resID)
	return idMatch[1], idMatch[2], nil
}

func dhcpResourceID(itf, mac string) string {
	return fmt.Sprintf("%s/%s", itf, mac)
}

func resourceDhcpStaticMappingCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	dhcp := pconf.DHCP
	lock := pconf.Mutex

	lock.Lock()

	err := dhcp.GetLeases()
	if err != nil {
		lock.Unlock()
		return fmt.Errorf("unable to retrieve configured DHCP leases")
	}

	// check if MAC address is not already registered
	mac := d.Get(KeyMAC).(string)
	for _, l := range dhcp.Leases {
		if l.MAC == mac {
			lock.Unlock()
			return fmt.Errorf("mapping for this MAC already exists")
		}
	}

	// create a new lease
	itf := d.Get(KeyInterface).(string)
	lease := DHCPLease{
		Interface: itf,
		IP:        d.Get(KeyIP).(string),
		MAC:       mac,
		Hostname:  d.Get(KeyName).(string),
	}

	err = dhcp.CreateLease(&lease)
	if err != nil {
		lock.Unlock()
		return err
	}

	time.Sleep(100 * time.Millisecond)

	// set resource ID accordingly
	d.SetId(dhcpResourceID(itf, mac))

	// read out resource again
	lock.Unlock()
	err = resourceDhcpStaticMappingRead(d, meta)

	return err
}

func resourceDhcpStaticMappingRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	lock := pconf.Mutex
	dhcp := pconf.DHCP

	lock.Lock()
	defer lock.Unlock()

	itf, mac, err := parseDhcpResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	// fetch leases
	err = dhcp.GetLeases()
	if err != nil {
		return fmt.Errorf("unable to retrieve configured DHCP leases")
	}

	// check if MAC address exists
	found := 0
	var lease *DHCPLease
	for _, l := range dhcp.Leases {
		if l.MAC == mac {
			found = 1
			lease = &l
			break
		}
	}
	if found == 0 {
		return fmt.Errorf("mapping for this ID do not exists (id: %s)", d.Id())
	}

	d.SetId(dhcpResourceID(itf, lease.MAC))

	err = d.Set(KeyInterface, lease.Interface)
	if err != nil {
		return err
	}
	err = d.Set(KeyIP, lease.IP)
	if err != nil {
		return err
	}
	err = d.Set(KeyName, lease.Hostname)
	if err != nil {
		return err
	}
	err = d.Set(KeyMAC, lease.MAC)
	if err != nil {
		return err
	}

	return nil
}

func resourceDhcpStaticMappingDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	lock := pconf.Mutex
	dhcp := pconf.DHCP

	lock.Lock()
	defer lock.Unlock()

	itf, mac, err := parseDhcpResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	err = dhcp.GetLeases()
	if err != nil {
		return fmt.Errorf("unable to retrieve configured DHCP leases")
	}

	// check if MAC address exists
	found := 0
	for _, l := range dhcp.Leases {
		if l.MAC == mac {
			found = 1
			break
		}
	}
	if found == 0 {
		return fmt.Errorf("mapping for this ID do not exists (id: %s)", d.Id())
	}

	// delete an existing lease
	lease := DHCPLease{
		Interface: itf,
		IP:        d.Get(KeyIP).(string),
		MAC:       mac,
		Hostname:  d.Get(KeyName).(string),
	}

	err = dhcp.DeleteLease(&lease)
	if err != nil {
		return err
	}

	return nil
}

func resourceDhcpStaticMappingUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	dhcp := pconf.DHCP
	lock := pconf.Mutex

	lock.Lock()
	defer lock.Unlock()

	itf, mac, err := parseDhcpResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	err = dhcp.GetLeases()
	if err != nil {
		return fmt.Errorf("unable to retrieve configured DHCP leases")
	}

	// check if MAC address exists
	found := 0
	for _, l := range dhcp.Leases {
		if l.MAC == mac {
			found = 1
			break
		}
	}
	if found == 0 {
		return fmt.Errorf("mapping for this ID do not exists (id: %s)", d.Id())
	}

	// define old lease
	oldLease := DHCPLease{
		Interface: itf,
		MAC:       mac,
	}

	// define new lease
	newLease := DHCPLease{
		Interface: itf,
		IP:        d.Get(KeyIP).(string),
		MAC:       mac,
		Hostname:  d.Get(KeyName).(string),
	}

	err = dhcp.EditLease(&oldLease, &newLease)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	return nil
}
