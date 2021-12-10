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

func parseDhcpResourceID(resID string) (string, string, error) {
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

	// create a new static mapping
	iface := d.Get(KeyInterface).(string)
	mac := d.Get(KeyMAC).(string)
	m := StaticMapping{
		Interface: iface,
		IP:        d.Get(KeyIP).(string),
		MAC:       mac,
		Hostname:  d.Get(KeyName).(string),
	}

	err := dhcp.CreateStaticMapping(&m)
	if err != nil {
		lock.Unlock()
		return err
	}

	time.Sleep(100 * time.Millisecond)

	// set resource ID accordingly
	d.SetId(dhcpResourceID(iface, mac))

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

	iface, mac, err := parseDhcpResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	m := StaticMapping{
		Interface: iface,
		MAC:       mac,
	}

	// read out DHCP information
	err = dhcp.ReadStaticMapping(&m)
	if err != nil {
		d.SetId("")
		return err
	}

	// set Terraform resource ID
	d.SetId(dhcpResourceID(iface, m.MAC))

	// set object params
	d.Set(KeyInterface, m.Interface)
	d.Set(KeyIP, m.IP)
	d.Set(KeyName, m.Hostname)
	d.Set(KeyMAC, m.MAC)

	return nil
}

func resourceDhcpStaticMappingDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	lock := pconf.Mutex
	dhcp := pconf.DHCP

	lock.Lock()
	defer lock.Unlock()

	iface, mac, err := parseDhcpResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	// delete an existing mapping
	m := StaticMapping{
		Interface: iface,
		MAC:       mac,
	}

	err = dhcp.DeleteStaticMapping(&m)
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

	iface, mac, err := parseDhcpResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	// updated mapping
	m := StaticMapping{
		Interface: iface,
		IP:        d.Get(KeyIP).(string),
		MAC:       mac,
		Hostname:  d.Get(KeyName).(string),
	}

	err = dhcp.UpdateStaticMapping(&m)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	return nil
}
