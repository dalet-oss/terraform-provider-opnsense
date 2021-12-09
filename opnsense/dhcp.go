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

// DHCPEntryStartingRow exposes the HTML row where static maps actually start from
const DHCPEntryStartingRow = 2

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

const (
	// DHCPStaticARP refers to the HTML table field for DHCP static map creation/edition
	DHCPStaticARP = "Static ARP"
	// DHCPMAC refers to the HTML table field for DHCP static map creation/edition
	DHCPMAC = "MAC address"
	// DHCPIP refers to the HTML table field for DHCP static map creation/edition
	DHCPIP = "IP address"
	// DHCPHostname refers to the HTML table field for DHCP static map creation/edition
	DHCPHostname = "Hostname"
	// DHCPDescription refers to the HTML table field for DHCP static map creation/edition
	DHCPDescription = "Description"
)

// DHCPStaticLeases abstract OPNSense DHCP Lease page
type DHCPStaticLeases struct {
	RootURI string
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

// DHCPInterface abstracts a network interface
type DHCPInterface struct {
	Name      string
	FormName  string
	FormValue string
	Doc       *html.Node
}

// Authenticate allows authentication to OPNsense DHCP lease web page
func (dhcp *DHCPStaticLeases) Authenticate(rootURI, user, password string) error {

	dhcp.RootURI = rootURI
	dhcp.Session = requests.Requests()

	// do a basic query
	resp, err := dhcp.Session.Get(dhcp.RootURI)
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
	resp, err = dhcp.Session.Post(dhcp.RootURI, data)
	if err != nil {
		return err
	}

	return nil
}

// GetLeasePage renders the OPNsense DHCP lease web page
func (dhcp *DHCPStaticLeases) GetLeasePage() error {

	getURI := fmt.Sprintf("%s/status_dhcp_leases.php", dhcp.RootURI)
	resp, err := dhcp.Session.Get(getURI)
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

// GetLeasesFields extracts the HTML page leases headers
func (dhcp *DHCPStaticLeases) GetLeasesFields() {
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

	// Get Leases page
	err := dhcp.GetLeasePage()
	if err != nil {
		panic(err)
	}

	// start by retrieving list of available fields
	dhcp.GetLeasesFields()

	// count number of registered DHCP entries
	nodes, err := htmlquery.QueryAll(dhcp.Doc, "//tr")
	if err != nil {
		return err
	}
	entries := len(nodes)

	// clean-up any previous leases
	dhcp.Leases = nil

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

// Apply validates the configuration for a given interface and reload DHCP server
func (dhcp *DHCPStaticLeases) Apply(itf *DHCPInterface) error {
	// apply changes
	data := requests.Datas{
		itf.FormName: itf.FormValue,
		"apply":      "Apply changes",
		"if":         itf.Name,
	}

	applyURI := fmt.Sprintf("%s/services_dhcp.php?if=%s#", dhcp.RootURI, itf.Name)
	_, err := dhcp.Session.Post(applyURI, data)
	if err != nil {
		return err
	}
	return nil
}

// CreateEditLease creates or edit a static lease
func (dhcp *DHCPStaticLeases) CreateEditLease(lease *DHCPLease, editID int) error {

	itf := DHCPInterface{
		Name: lease.Interface,
	}

	// get the edit page to retrieve form secret values
	editURI := fmt.Sprintf("%s/services_dhcp_edit.php?if=%s", dhcp.RootURI, itf.Name)
	if editID != -1 {
		editURI = fmt.Sprintf("%s&id=%d", editURI, editID)
	}
	resp, err := dhcp.Session.Get(editURI)
	if err != nil {
		return err
	}
	page := strings.NewReader(resp.Text())
	itf.Doc, err = htmlquery.Parse(page)
	if err != nil {
		return err
	}

	q := fmt.Sprintf(`//div[@class="content-box"]//form//input`)
	n := htmlquery.FindOne(itf.Doc, q)
	itf.FormName = htmlquery.SelectAttr(n, "name")
	itf.FormValue = htmlquery.SelectAttr(n, "value")

	// create a new DHCP entry
	data := requests.Datas{
		itf.FormName: itf.FormValue,
		"mac":        lease.MAC,
		"cid":        lease.Hostname,
		"ipaddr":     lease.IP,
		"hostname":   lease.Hostname,
		"descr":      lease.Hostname,
		"Submit":     "Save",
		"if":         lease.Interface,
	}
	if editID != -1 {
		data["id"] = fmt.Sprintf("%d", editID)
	}

	resp, err = dhcp.Session.Post(editURI, data)
	if err != nil {
		return err
	}

	// apply changes
	err = dhcp.Apply(&itf)
	if err != nil {
		return err
	}

	return nil
}

// CreateLease creates a new static lease
func (dhcp *DHCPStaticLeases) CreateLease(lease *DHCPLease) error {

	err := dhcp.CreateEditLease(lease, -1)
	if err != nil {
		return err
	}

	return nil
}

// GetStaticFields extracts the HTML page leases headers for creation/edition
func (dhcp *DHCPStaticLeases) GetStaticFields(itf *DHCPInterface, start int) []string {
	q := fmt.Sprintf(`//table[@class="table table-striped"]//tr[%d]`, start)
	headers := htmlquery.FindOne(itf.Doc, q)
	fields := []string{}
	for child := headers.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			content := strings.TrimSpace(htmlquery.InnerText(child))
			if len(content) > 0 {
				fields = append(fields, content)
			}
		}
	}

	return fields
}

// GetStatic extracts a given DHCP map from OPNsense DHCP interface web page
func (dhcp *DHCPStaticLeases) GetStatic(Doc *html.Node, fields []string, field string) string {
	res := ""

	// find the requested field index in HTML table
	id := index(fields, field) + 1
	if id == -1 {
		return res
	}

	// XPath query to find the associated HTML node
	q := fmt.Sprintf(`//td[%d]//text()`, id)
	values, err := htmlquery.QueryAll(Doc, q)
	if err != nil {
		return res
	}

	// extract value
	for _, v := range values {
		if v.Type != html.TextNode {
			continue
		}
		content := strings.TrimSpace(htmlquery.InnerText(v))
		if field == DHCPMAC {
			re := "([0-9a-f]{2}(?::[0-9a-f]{2}){5})"
			matched, err := regexp.Match(re, []byte(content))
			if err != nil || !matched {
				continue
			}
		}
		res = res + content
	}

	return res
}

// GetStaticMapID provides the HTML TR position ID for the requested static map
func (dhcp *DHCPStaticLeases) GetStaticMapID(itf *DHCPInterface, lease *DHCPLease) (int, error) {

	mapID := -1

	// get the edit page to retrieve form secret values
	dhcpURI := fmt.Sprintf("%s/services_dhcp.php?if=%s#", dhcp.RootURI, itf.Name)
	resp, err := dhcp.Session.Get(dhcpURI)
	if err != nil {
		return mapID, err
	}
	page := strings.NewReader(resp.Text())
	itf.Doc, err = htmlquery.Parse(page)
	if err != nil {
		return mapID, err
	}

	// lookup for static fields types
	fields := dhcp.GetStaticFields(itf, DHCPEntryStartingRow)

	// XPath query to find the associated HTML node
	q := fmt.Sprintf(`//table[@class="table table-striped"]//tr`)
	rows, err := htmlquery.QueryAll(itf.Doc, q)
	if err != nil {
		return mapID, err
	}

	// retrieve all configured static DHCP maps
	for i := DHCPEntryStartingRow; i < len(rows); i++ {
		mac := dhcp.GetStatic(rows[i], fields, DHCPMAC)

		// ensure we find the right ID
		if lease.MAC == mac {
			mapID = i - DHCPEntryStartingRow
		}
	}

	if mapID == -1 {
		return mapID, fmt.Errorf("invalid dhcp map ID")
	}

	return mapID, nil
}

// EditLease modifies an already existing static lease
func (dhcp *DHCPStaticLeases) EditLease(orig, new *DHCPLease) error {

	itf := DHCPInterface{
		Name: orig.Interface,
	}

	// get lease map ID
	mapID, err := dhcp.GetStaticMapID(&itf, orig)
	if err != nil {
		return err
	}

	// we finally got the ID, now we can edit
	err = dhcp.CreateEditLease(new, mapID)
	if err != nil {
		return err
	}

	return nil
}

// DeleteLease destroy an existing static lease
func (dhcp *DHCPStaticLeases) DeleteLease(lease *DHCPLease) error {

	itf := DHCPInterface{
		Name: lease.Interface,
	}

	// get lease map ID
	mapID, err := dhcp.GetStaticMapID(&itf, lease)
	if err != nil {
		return err
	}

	// get the edit page to retrieve form secret values
	dhcpURI := fmt.Sprintf("%s/services_dhcp.php?if=%s", dhcp.RootURI, itf.Name)

	// destroy DHCP entry
	data := requests.Datas{
		"if":  lease.Interface,
		"id":  fmt.Sprintf("%d", mapID),
		"act": "del",
	}

	_, err = dhcp.Session.Post(dhcpURI, data)
	if err != nil {
		return err
	}

	// apply changes
	err = dhcp.Apply(&itf)
	if err != nil {
		return err
	}

	return nil
}
