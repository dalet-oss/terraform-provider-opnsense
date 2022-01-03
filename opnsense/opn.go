package opnsense

import (
	"fmt"
	"github.com/asmcos/requests"
	"net/http"
	"regexp"
)

// OPNSession abstracts OPNSense connection
type OPNSession struct {
	RootURI string
	Session *requests.Request
	Cookies []*http.Cookie
	CSRF    string
}

// Error throws custom errors
func (s *OPNSession) Error(err string) error {
	return fmt.Errorf(err)
}

// Authenticate allows authentication to OPNsense main web page
func (s *OPNSession) Authenticate(rootURI, user, password string) error {

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
func (s *OPNSession) IsAuthenticated() error {
	if s.CSRF == "" {
		return fmt.Errorf("can't establish a session to OPNSense")
	}
	return nil
}
