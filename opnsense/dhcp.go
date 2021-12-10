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

const (
	// DHCPServiceURI is the WebUI service URI
	DHCPServiceURI = "/services_dhcp.php"
	// DHCPServiceEditURI is the WebUI service edit URI
	DHCPServiceEditURI = "/services_dhcp_edit.php"
)

const (
	// ErrNoMappings is thrown when no mapping can be found
	ErrNoMappings = "unable to retrieve list of static mappings"
	// ErrMACExists is thrown when a mapping already exists for this MAC address
	ErrMACExists = "mapping for this MAC already exists"
	// ErrNoSuchMAC is thrown if no mapping can be found for the specific Interface/MAC couple
	ErrNoSuchMAC = "mapping doesn't exists for this MAC address"
)

// DHCPSession abstracts OPNSense DHCP Interface
type DHCPSession struct {
	RootURI string
	Session *requests.Request
	Cookies []*http.Cookie
	CSRF    string
	Fields  []string
}

// StaticMapping abstracts a static DHCP mapping entry
type StaticMapping struct {
	ID        int
	Interface string
	IP        string
	MAC       string
	Hostname  string
}

// Error throws custom errors
func (s *DHCPSession) Error(err string) error {
	return fmt.Errorf(err)
}

// Authenticate allows authentication to OPNsense DHCP lease web page
func (s *DHCPSession) Authenticate(rootURI, user, password string) error {

	s.RootURI = rootURI
	s.Session = requests.Requests()

	// do a basic query
	resp, err := s.Session.Get(s.RootURI)
	if err != nil {
		return err
	}

	// fetch up cookies
	s.Cookies = resp.Cookies()

	// read CSRF token
	re := regexp.MustCompile(`"X-CSRFToken", "(.*)" \);`)
	csrf := re.FindSubmatch([]byte(resp.Text()))
	s.CSRF = string(csrf[1])

	// re-try with authentication
	data := requests.Datas{
		"login":       "Login",
		"usernamefld": user,
		"passwordfld": password,
	}
	s.Session.Header.Set("X-CSRFToken", s.CSRF)
	resp, err = s.Session.Post(s.RootURI, data)
	if err != nil {
		return err
	}

	return nil
}

// IsAuthenticated throws an error if no session has been initialized
func (s *DHCPSession) IsAuthenticated() error {
	if s.CSRF == "" {
		return fmt.Errorf("can't establish a session to OPNSense")
	}
	return nil
}

// GetStaticFieldNames extracts the HTML page leases headers for creation/edition
func (s *DHCPSession) GetStaticFieldNames(node *html.Node, start int) {
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

// GetStaticMappingField extracts a given DHCP mapping from OPNsense DHCP interface web page
func (s *DHCPSession) GetStaticMappingField(node *html.Node, f string) string {
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
		if f == DHCPMAC {
			re := "([0-9a-f]{2}(?::[0-9a-f]{2}){5})"
			matched, err := regexp.Match(re, []byte(content))
			if err != nil || !matched {
				continue
			}
		}
		if f == DHCPHostname {
			if content == "" {
				content = "default"
			}
		}
		res = res + content
	}

	return res
}

// GetAllInterfaceStaticMappings retrieves the list of all configured static mappings for a given interface
func (s *DHCPSession) GetAllInterfaceStaticMappings(iface string) ([]StaticMapping, error) {

	entries := []StaticMapping{}

	// check for proper authentication
	err := s.IsAuthenticated()
	if err != nil {
		return entries, err
	}

	// read out the service page
	dhcpURI := fmt.Sprintf("%s%s?if=%s", s.RootURI, DHCPServiceURI, iface)
	resp, err := s.Session.Get(dhcpURI)
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
	s.GetStaticFieldNames(doc, DHCPEntryStartingRow)

	// XPath query to find all table rows
	q := fmt.Sprintf(`//table[@class="table table-striped"]//tr`)
	rows, err := htmlquery.QueryAll(doc, q)
	if err != nil {
		return entries, err
	}

	// retrieve all configured static DHCP mappings
	for i := DHCPEntryStartingRow; i < len(rows); i++ {
		r := rows[i]
		m := StaticMapping{
			ID:        i - DHCPEntryStartingRow,
			Interface: iface,
			IP:        s.GetStaticMappingField(r, DHCPIP),
			MAC:       s.GetStaticMappingField(r, DHCPMAC),
			Hostname:  s.GetStaticMappingField(r, DHCPHostname),
		}
		entries = append(entries, m)
	}

	return entries, nil
}

// Apply validates the configuration for a given interface and reload DHCP server
func (s *DHCPSession) Apply(iface, formName, formValue string) error {
	// apply changes
	data := requests.Datas{
		"apply": "Apply changes",
		"if":    iface,
	}
	if formName != "" || formValue != "" {
		data[formName] = formValue
	}

	applyURI := fmt.Sprintf("%s%s?if=%s", s.RootURI, DHCPServiceURI, iface)
	_, err := s.Session.Post(applyURI, data)
	if err != nil {
		return err
	}
	return nil
}

// CreateOrEdit creates or edit a static mapping
func (s *DHCPSession) CreateOrEdit(m *StaticMapping) error {

	// get the edit page to retrieve form secret values
	editURI := fmt.Sprintf("%s%s?if=%s", s.RootURI, DHCPServiceEditURI, m.Interface)
	if m.ID != -1 {
		editURI = fmt.Sprintf("%s&id=%d", editURI, m.ID)
	}
	resp, err := s.Session.Get(editURI)
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
		formName:   formValue,
		"mac":      m.MAC,
		"cid":      m.Hostname,
		"ipaddr":   m.IP,
		"hostname": m.Hostname,
		"descr":    m.Hostname,
		"Submit":   "Save",
		"if":       m.Interface,
	}
	if m.ID != -1 {
		data["id"] = fmt.Sprintf("%d", m.ID)
	}

	resp, err = s.Session.Post(editURI, data)
	if err != nil {
		return err
	}

	// apply changes
	err = s.Apply(m.Interface, formName, formValue)
	if err != nil {
		return err
	}

	return nil
}

// FindMappingByMAC retrieves all entries for a given interface and select the one that matches
func (s *DHCPSession) FindMappingByMAC(m *StaticMapping) (*StaticMapping, error) {

	// retrieves existing mappings
	entries, err := s.GetAllInterfaceStaticMappings(m.Interface)
	if err != nil {
		return nil, err
	}

	// check if an entry existing for this MAC
	for _, e := range entries {
		// we found it
		if e.MAC == m.MAC {
			return &e, nil
		}
	}

	return nil, s.Error(ErrNoSuchMAC)
}

// CreateStaticMapping creates a new static lease
func (s *DHCPSession) CreateStaticMapping(m *StaticMapping) error {

	e, err := s.FindMappingByMAC(m)

	// check if the MAC address is not already registered
	if e != nil {
		return s.Error(ErrMACExists)
	}

	// create the mapping entry
	m.ID = -1
	err = s.CreateOrEdit(m)
	if err != nil {
		return err
	}

	return nil
}

// ReadStaticMapping retrieves mapping information for a specified Interface/MAC couple
func (s *DHCPSession) ReadStaticMapping(m *StaticMapping) error {

	// check if an entry existing for this Interface/MAC couple
	e, err := s.FindMappingByMAC(m)
	if e == nil {
		return err
	}

	// assign values accordingly
	m.ID = e.ID
	m.IP = e.IP
	m.Hostname = e.Hostname

	return nil
}

// UpdateStaticMapping modifies an already existing static mapping
func (s *DHCPSession) UpdateStaticMapping(m *StaticMapping) error {

	// check if an entry existing for this Interface/MAC couple
	e, err := s.FindMappingByMAC(m)
	if e == nil {
		return err
	}

	// update the mapping entry
	m.ID = e.ID
	err = s.CreateOrEdit(m)
	if err != nil {
		return err
	}

	return nil
}

// DeleteStaticMapping destroy an existing static mapping
func (s *DHCPSession) DeleteStaticMapping(m *StaticMapping) error {

	// check if an entry existing for this Interface/MAC couple
	e, err := s.FindMappingByMAC(m)
	if e == nil {
		return err
	}

	// get the edit page to retrieve form secret values
	dhcpURI := fmt.Sprintf("%s%s?if=%s", s.RootURI, DHCPServiceURI, e.Interface)

	// destroy DHCP entry
	data := requests.Datas{
		"if":  e.Interface,
		"id":  fmt.Sprintf("%d", e.ID),
		"act": "del",
	}

	_, err = s.Session.Post(dhcpURI, data)
	if err != nil {
		return err
	}

	// apply changes
	err = s.Apply(e.Interface, "", "")
	if err != nil {
		return err
	}

	return nil
}
