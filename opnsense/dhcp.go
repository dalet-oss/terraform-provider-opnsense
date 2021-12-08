package opnsense

import (
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/asmcos/requests"
	"golang.org/x/net/html"
	"net/http"
	"regexp"
	"strings"
)

const (
	// DHCPFieldInterface refers to the HTML table field name for DHCP interface description
	DHCPFieldInterface = "Interface"
	// DHCPFieldIP refers to the HTML table field name for DHCP IP address description
	DHCPFieldIP = "IP address"
	// DHCPFieldMAC refers to the HTML table field name for DHCP MAc address description
	DHCPFieldMAC = "MAC address"
	// DHCPFieldHostname refers to the HTML table field name for DHCP hostname description
	DHCPFieldHostname = "Hostname"
	// DHCPFieldDescription refers to the HTML table field name for DHCP description
	DHCPFieldDescription = "Description"
	// DHCPFieldStart refers to the HTML table field name for DHCP lease start time description
	DHCPFieldStart = "Start"
	// DHCPFieldEnd refers to the HTML table field name for DHCP lease end time description
	DHCPFieldEnd = "End"
	// DHCPFieldStatus refers to the HTML table field name for DHCP lease status description
	DHCPFieldStatus = "Status"
	// DHCPFieldLease refers to the HTML table field name for DHCP lease type description
	DHCPFieldLease = "Lease type"
)

// DHCPStaticLeases abstract OPNSense DHCP Lease page
type DHCPStaticLeases struct {
	URI     string
	Session *requests.Request
	Cookies []*http.Cookie
	CSRF    string
	Doc     *html.Node
	Fields  []string
	Leases  []DHCPLease
}

// DHCPLease abstract a given lease
type DHCPLease struct {
	Interface string
	IP        string
	MAC       string
	Hostname  string
}

// Authenticate allows authentication to OPNsense DHCP lease web page
func (dhcp *DHCPStaticLeases) Authenticate(rootURI, user, password string) error {

	dhcp.URI = fmt.Sprintf("%s/status_dhcp_leases.php", rootURI)
	dhcp.Session = requests.Requests()

	// do a basic query
	resp, err := dhcp.Session.Get(dhcp.URI)
	if err != nil {
		return err
	}

	// fetch up cookies
	dhcp.Cookies = resp.Cookies()

	// read CSRF token
	re := regexp.MustCompile(`"X-CSRFToken", "(.*)" \);`)
	csrf := re.FindSubmatch([]byte(resp.Text()))
	dhcp.CSRF = string(csrf[1])

	// re-try with authentication
	data := requests.Datas{
		"login":       "Login",
		"usernamefld": user,
		"passwordfld": password,
	}
	dhcp.Session.Header.Set("X-CSRFToken", dhcp.CSRF)
	resp, err = dhcp.Session.Post(dhcp.URI, data)
	if err != nil {
		return err
	}

	return nil
}

// GetHTMLPage renders the OPNsense DHCP lease web page
func (dhcp *DHCPStaticLeases) GetHTMLPage() error {
	resp, err := dhcp.Session.Get(dhcp.URI)
	if err != nil {
		return err
	}

	// parse HTML
	page := strings.NewReader(resp.Text())
	dhcp.Doc, err = htmlquery.Parse(page)
	if err != nil {
		return err
	}

	return nil
}

// GetFields extracts the HTML page leases headers
func (dhcp *DHCPStaticLeases) GetFields() {
	headers := htmlquery.FindOne(dhcp.Doc, "//tr[1]")
	dhcp.Fields = []string{}
	for child := headers.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			content := htmlquery.InnerText(child)
			if len(content) > 0 {
				dhcp.Fields = append(dhcp.Fields, content)
			}
		}
	}
}

// PrintLeases displays all configured leases
func (dhcp *DHCPStaticLeases) PrintLeases() {
	for i, l := range dhcp.Leases {
		fmt.Printf("Lease #%.2d: %s (%s) reserved for %s on %s\n", i, l.Hostname, l.IP, l.MAC, l.Interface)
	}
}

// GetLeases extracts all DHCP leases from OPNsense DHCP lease web page
func (dhcp *DHCPStaticLeases) GetLeases() error {

	// start by retrieving list of available fields
	dhcp.GetFields()

	// count number of registered DHCP entries
	nodes, err := htmlquery.QueryAll(dhcp.Doc, "//tr")
	if err != nil {
		return err
	}
	entries := len(nodes)

	// retrieve all configured static DHCP leases
	for i := 1; i < entries; i++ {
		lease := DHCPLease{}
		lease.Interface = dhcp.Get(i, DHCPFieldInterface)
		lease.IP = dhcp.Get(i, DHCPFieldIP)
		lease.MAC = dhcp.Get(i, DHCPFieldMAC)
		lease.Hostname = dhcp.Get(i, DHCPFieldHostname)
		dhcp.Leases = append(dhcp.Leases, lease)
	}

	return nil
}

// Get extracts a given DHCP lease from OPNsense DHCP lease web page
func (dhcp *DHCPStaticLeases) Get(idx int, field string) string {
	res := ""

	// find the requested field index in HTML table
	id := index(dhcp.Fields, field) + 1
	if id == -1 {
		return res
	}

	// XPath query to find the associated HTML node
	q := fmt.Sprintf(`//table[@class="table table-striped"]//tbody//tr[%d]//td[%d]//text()`, idx, id)
	values, err := htmlquery.QueryAll(dhcp.Doc, q)
	if err != nil {
		return res
	}

	// extract value
	for _, v := range values {
		if v.Type != html.TextNode {
			continue
		}
		content := htmlquery.InnerText(v)
		if field == DHCPFieldMAC {
			re := "([0-9a-f]{2}(?::[0-9a-f]{2}){5})"
			matched, err := regexp.Match(re, []byte(content))
			if err != nil || !matched {
				continue
			}
			content = strings.TrimSpace(content)
		}
		if field == DHCPFieldHostname {
			if content == "" {
				content = "default"
			}
		}
		res = res + content
	}

	return res
}
