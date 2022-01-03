package opnsense

import (
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/asmcos/requests"
	"golang.org/x/net/html"
	"strings"
)

// DNSEntryStartingRow exposes the HTML row where static maps actually start from
const DNSEntryStartingRow = 2

const (
	// DNSHost refers to the HTML table field for DNS host entry creation/edition
	DNSHost = "Host"
	// DNSDomain refers to the HTML table field for DNS host entry creation/edition
	DNSDomain = "Domain"
	// DNSType refers to the HTML table field for DNS host entry creation/edition
	DNSType = "Type"
	// DNSValue refers to the HTML table field for DNS host entry creation/edition
	DNSValue = "Value"
	// DNSDescription refers to the HTML table field for DNS host entry creation/edition
	DNSDescription = "Description"
)

const (
	// DNSServiceURI is the WebUI service URI
	DNSServiceURI = "/services_unbound_overrides.php"
	// DNSServiceEditURI is the WebUI service edit URI
	DNSServiceEditURI = "/services_unbound_host_edit.php"
)

const (
	// ErrDNSNoEntries is thrown when no entry can be found
	ErrDNSNoEntries = "unable to retrieve list of DNS host overrides"
	// ErrDNSHostExists is thrown when an entry already exists for this host override
	ErrDNSHostExists = "DNS override for this host already exists"
	// ErrDNSNoSuchEntry is thrown if no host override entry can be found
	ErrDNSNoSuchEntry = "host override entry doesn't exists"
)

// DNSSession abstracts OPNSense UnboundDNS Overrides
type DNSSession struct {
	OPN    *OPNSession
	Fields []string
}

// DNSHostEntry abstracts a DNS Host override
type DNSHostEntry struct {
	ID     int
	Type   string
	Host   string
	Domain string
	IP     string
}

///////////////////////
// Private Functions //
///////////////////////

// GetStaticFieldNames extracts the HTML page host overrides headers for creation/edition
func (s *DNSSession) GetStaticFieldNames(node *html.Node, start int) {
	if len(s.Fields) > 0 {
		// already filled-in, no need to go any further
		return
	}

	q := fmt.Sprintf(`//table[@class="table table-striped"]//tr[%d]`, start)
	headers := htmlquery.FindOne(node, q)
	s.Fields = []string{}
	for child := headers.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			content := strings.TrimSpace(htmlquery.InnerText(child))
			if len(content) > 0 {
				s.Fields = append(s.Fields, content)
			}
		}
	}
}

// GetStaticMappingField extracts a given DNS host override entry from OPNsense DNS overrides web page
func (s *DNSSession) GetStaticMappingField(node *html.Node, f string) string {
	res := ""

	// find the requested field index in HTML table
	id := index(s.Fields, f) + 1
	if id == -1 {
		return res
	}

	// XPath query to find the associated HTML node
	q := fmt.Sprintf(`//td[%d]//text()`, id)
	values, err := htmlquery.QueryAll(node, q)
	if err != nil {
		return res
	}

	// extract value
	for _, v := range values {
		if v.Type != html.TextNode {
			continue
		}
		content := strings.TrimSpace(htmlquery.InnerText(v))
		res = res + content
	}

	return res
}

// GetAllHostEntries retrieves the list of all configured DNS host overrides
func (s *DNSSession) GetAllHostEntries() ([]DNSHostEntry, error) {

	entries := []DNSHostEntry{}

	// check for proper authentication
	err := s.OPN.IsAuthenticated()
	if err != nil {
		return entries, err
	}

	// read out the service page
	dnsURI := fmt.Sprintf("%s%s", s.OPN.RootURI, DNSServiceURI)
	resp, err := s.OPN.Session.Get(dnsURI)
	if err != nil {
		return entries, err
	}

	// get HTML
	page := strings.NewReader(resp.Text())
	doc, err := htmlquery.Parse(page)
	if err != nil {
		return entries, err
	}

	// lookup for static fields types
	s.GetStaticFieldNames(doc, DNSEntryStartingRow)

	// XPath query to find all table rows
	q := fmt.Sprintf(`//table[@class="table table-striped"]//tr`)
	rows, err := htmlquery.QueryAll(doc, q)
	if err != nil {
		return entries, err
	}

	// retrieve all configured DNS host override entries
	for i := DNSEntryStartingRow; i < len(rows); i++ {
		r := rows[i]
		e := DNSHostEntry{
			ID:     i - DNSEntryStartingRow,
			Type:   s.GetStaticMappingField(r, DNSType),
			Host:   s.GetStaticMappingField(r, DNSHost),
			Domain: s.GetStaticMappingField(r, DNSDomain),
			IP:     s.GetStaticMappingField(r, DNSValue),
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// HostsMatch compares if 2 host entries are alike
func (s *DNSSession) HostsMatch(e1, e2 *DNSHostEntry) bool {
	if (e1.Host == e2.Host) && (e1.Domain == e2.Domain) && (e1.Type == e2.Type) && (e1.IP == e2.IP) {
		return true
	}
	return false
}

// FindHostEntry retrieves all entries select the one that matches
func (s *DNSSession) FindHostEntry(h *DNSHostEntry) (*DNSHostEntry, error) {

	// retrieves existing host entries
	entries, err := s.GetAllHostEntries()
	if err != nil {
		return nil, err
	}

	// check if an entry exists
	for _, e := range entries {
		// we found it
		if s.HostsMatch(h, &e) {
			return &e, nil
		}
	}

	return nil, s.OPN.Error(ErrDNSNoSuchEntry)
}

// FindHostEntryByID retrieves all entries select the one that matches the ID
func (s *DNSSession) FindHostEntryByID(id int) (*DNSHostEntry, error) {

	// retrieves existing host entries
	entries, err := s.GetAllHostEntries()
	if err != nil {
		return nil, err
	}

	// check if an entry exists
	for _, e := range entries {
		// we found it
		if e.ID == id {
			return &e, nil
		}
	}

	return nil, s.OPN.Error(ErrDNSNoSuchEntry)
}

// Apply validates the configuration and reload DNS server
func (s *DNSSession) Apply(formName, formValue string) error {
	// apply changes
	data := requests.Datas{
		"apply": "Apply changes",
	}
	if formName != "" || formValue != "" {
		data[formName] = formValue
	}

	applyURI := fmt.Sprintf("%s%s", s.OPN.RootURI, DNSServiceURI)
	_, err := s.OPN.Session.Post(applyURI, data)
	if err != nil {
		return err
	}
	return nil
}

// CreateOrEdit creates or edit an host override entry
func (s *DNSSession) CreateOrEdit(e *DNSHostEntry) error {

	// get the edit page to retrieve form secret values
	editURI := fmt.Sprintf("%s%s", s.OPN.RootURI, DNSServiceEditURI)
	if e.ID != -1 {
		editURI = fmt.Sprintf("%s&id=%d", editURI, e.ID)
	}
	resp, err := s.OPN.Session.Get(editURI)
	if err != nil {
		return err
	}

	// get HTML
	page := strings.NewReader(resp.Text())
	doc, err := htmlquery.Parse(page)
	if err != nil {
		return err
	}

	// get form runtime values
	q := fmt.Sprintf(`//div[@class="content-box"]//form//input`)
	n := htmlquery.FindOne(doc, q)
	formName := htmlquery.SelectAttr(n, "name")
	formValue := htmlquery.SelectAttr(n, "value")

	// create a new DHCP entry
	data := requests.Datas{
		formName: formValue,
		"host":   e.Host,
		"domain": e.Domain,
		"rr":     e.Type,
		"ip":     e.IP,
		"descr":  "",
		"Submit": "Save",
	}
	if e.ID != -1 {
		data["id"] = fmt.Sprintf("%d", e.ID)
	}

	resp, err = s.OPN.Session.Post(editURI, data)
	if err != nil {
		return err
	}

	// apply changes
	err = s.Apply(formName, formValue)
	if err != nil {
		return err
	}

	return nil
}

//////////////////////
// Public Functions //
//////////////////////

// CreateHostOverride creates a new DNS host override entry
func (s *DNSSession) CreateHostOverride(h *DNSHostEntry) error {

	e, err := s.FindHostEntry(h)

	// check if the host override is not already registered
	if e != nil {
		return s.OPN.Error(ErrDNSHostExists)
	}

	// create the mapping entry
	h.ID = -1
	err = s.CreateOrEdit(h)
	if err != nil {
		return err
	}

	return nil
}

// ReadHostOverride retrieves DNS information for a specified host
func (s *DNSSession) ReadHostOverride(h *DNSHostEntry) error {

	// check if an entry exists
	e, err := s.FindHostEntry(h)
	if e == nil {
		return err
	}

	// assign values accordingly
	h.ID = e.ID

	return nil
}

// UpdateHostOverride modifies an already existing host override
func (s *DNSSession) UpdateHostOverride(h *DNSHostEntry) error {

	// check if an entry exists for this specific ID
	e, err := s.FindHostEntryByID(h.ID)
	if e == nil {
		return err
	}

	// update the mapping entry
	h.ID = e.ID
	err = s.CreateOrEdit(h)
	if err != nil {
		return err
	}

	return nil
}

// DeleteHostOverride destroy an existing DNS host entry
func (s *DNSSession) DeleteHostOverride(h *DNSHostEntry) error {

	// check if an entry exists
	e, err := s.FindHostEntry(h)
	if e == nil {
		return err
	}

	// get the DNS page to retrieve form secret values
	dnsURI := fmt.Sprintf("%s%s", s.OPN.RootURI, DNSServiceURI)

	// destroy DNS host entry
	data := requests.Datas{
		"id":  fmt.Sprintf("%d", e.ID),
		"act": "del",
	}

	_, err = s.OPN.Session.Post(dnsURI, data)
	if err != nil {
		return err
	}

	// apply changes
	err = s.Apply("", "")
	if err != nil {
		return err
	}

	return nil
}
