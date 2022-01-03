package opnsense

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

const (
	// KeyDNSType corresponds to the associated resource schema key
	KeyDNSType = "type"
	// KeyDNSHost corresponds to the associated resource schema key
	KeyDNSHost = "host"
	// KeyDNSDomain corresponds to the associated resource schema key
	KeyDNSDomain = "domain"
	// KeyDNSIP corresponds to the associated resource schema key
	KeyDNSIP = "ip"
)

func resourceOpnDNSHostOverride() *schema.Resource {
	return &schema.Resource{
		Create: resourceDNSHostOverrideCreate,
		Read:   resourceDNSHostOverrideRead,
		Update: resourceDNSHostOverrideUpdate,
		Delete: resourceDNSHostOverrideDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			KeyDNSType: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyDNSHost: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyDNSDomain: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.All(validation.StringIsNotEmpty, validation.StringIsNotWhiteSpace),
			},
			KeyDNSIP: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsIPAddress,
			},
		},
	}
}

var dnsRsID = regexp.MustCompile("([^/]+)/([^/]+)/([^/]+)/([^/]+)/([^/]+)")

func parseDNSResourceID(resID string) (*DNSHostEntry, error) {
	e := DNSHostEntry{}

	if !dnsRsID.MatchString(resID) {
		return &e, fmt.Errorf("invalid resource format: %s. must be type/host/domain/ip/id", resID)
	}
	idMatch := dnsRsID.FindStringSubmatch(resID)
	e.Type = idMatch[1]
	e.Host = idMatch[2]
	e.Domain = idMatch[3]
	e.IP = idMatch[4]
	e.ID, _ = strconv.Atoi(idMatch[5])

	return &e, nil
}

func dnsResourceID(e *DNSHostEntry) string {
	return fmt.Sprintf("%s/%s/%s/%s/%d", e.Type, e.Host, e.Domain, e.IP, e.ID)
}

func resourceDNSHostOverrideCreate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	dns := pconf.DNS
	lock := pconf.Mutex

	lock.Lock()

	// create a new host override
	e := DNSHostEntry{
		Type:   d.Get(KeyDNSType).(string),
		Host:   d.Get(KeyDNSHost).(string),
		Domain: d.Get(KeyDNSDomain).(string),
		IP:     d.Get(KeyDNSIP).(string),
	}

	err := dns.CreateHostOverride(&e)
	if err != nil {
		lock.Unlock()
		return err
	}

	time.Sleep(100 * time.Millisecond)

	// set resource ID accordingly
	d.SetId(dnsResourceID(&e))

	// read out resource again
	lock.Unlock()
	err = resourceDNSHostOverrideRead(d, meta)

	return err
}

func resourceDNSHostOverrideRead(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	lock := pconf.Mutex
	dns := pconf.DNS

	lock.Lock()
	defer lock.Unlock()

	e, err := parseDNSResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	// read out DNS Host information
	err = dns.ReadHostOverride(e)
	if err != nil {
		d.SetId("")
		return err
	}

	// set Terraform resource ID
	d.SetId(dnsResourceID(e))

	// set object params
	d.Set(KeyDNSType, e.Type)
	d.Set(KeyDNSHost, e.Host)
	d.Set(KeyDNSDomain, e.Domain)
	d.Set(KeyDNSIP, e.IP)

	return nil
}

func resourceDNSHostOverrideUpdate(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	lock := pconf.Mutex
	dns := pconf.DNS

	lock.Lock()

	e, err := parseDNSResourceID(d.Id())
	if err != nil {
		d.SetId("")
		lock.Unlock()
		return err
	}

	// updated entry
	e.IP = d.Get(KeyDNSIP).(string)

	err = dns.UpdateHostOverride(e)
	if err != nil {
		lock.Unlock()
		return err
	}

	time.Sleep(100 * time.Millisecond)

	// read out resource again
	lock.Unlock()
	err = resourceDNSHostOverrideRead(d, meta)

	return nil
}

func resourceDNSHostOverrideDelete(d *schema.ResourceData, meta interface{}) error {
	pconf := meta.(*ProviderConfiguration)
	lock := pconf.Mutex
	dns := pconf.DNS

	lock.Lock()
	defer lock.Unlock()

	e, err := parseDNSResourceID(d.Id())
	if err != nil {
		d.SetId("")
		return err
	}

	err = dns.DeleteHostOverride(e)
	if err != nil {
		return err
	}

	return nil
}
