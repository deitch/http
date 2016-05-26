package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type User struct {
	user string
	pass string
}

var authUser = User{
	user: "john",
	pass: "somepass",
}
var realmUser = User{
	user: "rjohn",
	pass: "rsomepass",
}
var goodToken = "1234567890abcdefghijklmnopqrstuvwxyz"
var bearerHeader string

var ts, realmTS *httptest.Server
var u, realm string

func TestMain(m *testing.M) {
	flag.Parse()
	// set up Web server and realm auth server
	realmTS = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok && user == realmUser.user && pass == realmUser.pass {
			w.WriteHeader(200)
			fmt.Fprint(w, "{\"token\":\""+goodToken+"\"}")
		} else {
			w.WriteHeader(401)
		}
	}))
	bearerHeader = "Bearer realm=\"" + realmTS.URL + "\",service=\"foo.bar.me\""

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//requestHeaders := r.Header
		if r.Method == "GET" && r.URL.Path == "/foo" {
			w.Header().Add("x-my-header", "here")
			w.WriteHeader(200)
			fmt.Fprint(w, "bar")
		}
		if r.Method == "HEAD" && r.URL.Path == "/foo" {
			w.Header().Add("x-my-header", "here")
			w.WriteHeader(200)
		}
		if r.URL.Path == "/headers" {
			w.WriteHeader(200)
			// just reflect all of the headers back into the body
			r.Header.Write(w)
		}
		if r.URL.Path == "/auth" {
			// check if body contained auth, and it was correct
			user, pass, ok := r.BasicAuth()
			if ok && user == authUser.user && pass == authUser.pass {
				w.WriteHeader(200)
				fmt.Fprint(w, "auth")
			} else {
				w.WriteHeader(401)
			}
		}
		if r.URL.Path == "/realm" {
			token := r.Header.Get("Authorization")
			if token == "" {
				w.Header().Add("Www-Authenticate", bearerHeader)
				w.WriteHeader(401)
			} else if token == "Bearer "+goodToken {
				w.WriteHeader(200)
				fmt.Fprint(w, "realm")
			} else {
				w.Header().Add("Www-Authenticate", "Bearer invalid_token")
				w.WriteHeader(401)
			}
		}
		if r.Method == "POST" {
			w.Header().Add("Location", ts.URL+"/foo/123")
			w.WriteHeader(201)
			io.Copy(w, r.Body)
		}
		if r.Method == "PUT" && r.URL.Path == "/foo/123" {
			w.WriteHeader(200)
			io.Copy(w, r.Body)
		}
		if r.Method == "PATCH" && r.URL.Path == "/foo/123" {
			w.WriteHeader(200)
			io.Copy(w, r.Body)
		}
		if r.Method == "DELETE" && r.URL.Path == "/foo/123" {
			w.WriteHeader(204)
		}

	}))
	u = ts.URL

	os.Exit(m.Run())
}

/*
 * test request/response cycle
 */
func TestInvalidUrl(t *testing.T) {
	_, err := doreq("GET", "a.b.c.foo", nil, "", "", "")
	if err == nil {
		t.Error("Expected response to have an error")
	}
}

func TestInvalidUser(t *testing.T) {
	resp, err := doreq("GET", ts.URL+"/auth", nil, "jim:foo", "", "")
	if err != nil {
		t.Error("Expected response to GET to have no error")
	}
	if resp.StatusCode != 401 {
		t.Error("Expected response status code to be 401 instead of ", resp.StatusCode)
	}
}

func TestUserNoPass(t *testing.T) {
	resp, err := doreq("GET", ts.URL+"/auth", nil, "", "", "")
	if err != nil {
		t.Error("Expected response to GET to have no error")
	}
	if resp.StatusCode != 401 {
		t.Error("Expected response status code to be 401 instead of ", resp.StatusCode)
	}
}

func TestValidUser(t *testing.T) {
	resp, err := doreq("GET", ts.URL+"/auth", nil, authUser.user+":"+authUser.pass, "", "")
	if err != nil {
		t.Error("Expected response to GET to have no error")
	}
	if resp.StatusCode != 200 {
		t.Error("Expected response status code to be 200 instead of ", resp.StatusCode)
	}
	if !strings.Contains(getBody(resp), "auth") {
		t.Error("Body did not contain word 'auth'")
	}
}

func TestGet(t *testing.T) {
	resp, err := doreq("GET", ts.URL+"/foo", nil, "", "", "")
	if err != nil {
		t.Error("Expected response to GET to have no error")
	}
	if resp.StatusCode != 200 {
		t.Error("Expected response status code to GET to be 200 instead of ", resp.StatusCode)
	}
	if !strings.Contains(getBody(resp), "bar") {
		t.Error("Body did not contain word 'bar'")
	}
}

func TestPut(t *testing.T) {
	msg := "This is the message"
	resp, err := doreq("PUT", ts.URL+"/foo/123", nil, "", "", msg)
	if err != nil {
		t.Error("Expected response to PUT to have no error")
	}
	if resp.StatusCode != 200 {
		t.Error("Expected response status code to be 200 instead of ", resp.StatusCode)
	}
	body := getBody(resp)
	if body != msg {
		t.Error("Expected body to be '" + msg + "' instead of '" + body + "'")
	}
}

func TestPost(t *testing.T) {
	msg := "This is the message"
	elocn := "/foo/123"
	resp, err := doreq("POST", ts.URL+"/foo", nil, "", "", msg)
	if err != nil {
		t.Error("Expected response to POST to have no error")
	}
	if resp.StatusCode != 201 {
		t.Error("Expected response status code to be 201 instead of ", resp.StatusCode)
	}
	body := getBody(resp)
	if body != msg {
		t.Error("Expected body to be '" + msg + "' instead of '" + body + "'")
	}
	locn := resp.Header.Get("Location")
	if !strings.HasSuffix(locn, elocn) {
		t.Error("Expected location to end in '" + elocn + "' instead of '" + locn + "'")
	}
}

func TestPatch(t *testing.T) {
	msg := "This is the message"
	resp, err := doreq("PATCH", ts.URL+"/foo/123", nil, "", "", msg)
	if err != nil {
		t.Error("Expected response to PATCH to have no error")
	}
	if resp.StatusCode != 200 {
		t.Error("Expected response status code to be 200 instead of ", resp.StatusCode)
	}
	body := getBody(resp)
	if body != msg {
		t.Error("Expected body to be '" + msg + "' instead of '" + body + "'")
	}

}

func TestHead(t *testing.T) {
	resp, err := doreq("HEAD", ts.URL+"/foo", nil, "", "", "")
	if err != nil {
		t.Error("Expected response to HEAD to have no error")
	}
	if resp.StatusCode != 200 {
		t.Error("Expected response status code to be 200 instead of ", resp.StatusCode)
	}
	body := getBody(resp)
	if body != "" {
		t.Error("Body should be empty instead of " + body)
	}
}

func TestDelete(t *testing.T) {
	msg := "This is the message"
	resp, err := doreq("DELETE", ts.URL+"/foo/123", nil, "", "", msg)
	if err != nil {
		t.Error("Expected response to DELETE to have no error")
	}
	if resp.StatusCode != 204 {
		t.Error("Expected response status code to be 204 instead of ", resp.StatusCode)
	}
}

func TestNoFollowRealm(t *testing.T) {
	resp, err := doreq("GET", ts.URL+"/realm", nil, "", "", "")
	if err != nil {
		t.Error("Expected response to GET to have no error")
	}
	if resp.StatusCode != 401 {
		t.Error("Expected response status code to be 401 instead of ", resp.StatusCode)
	}
	authHeader := resp.Header.Get("Www-Authenticate")
	if authHeader != bearerHeader {
		t.Error("Expected www-authenticate header to be " + bearerHeader + " instead of '" + authHeader + "'")
	}
}

func TestFollowRealmBadAuth(t *testing.T) {
	_, err := doreq("GET", ts.URL+"/realm", nil, "", "asasas:asqwhqhsqs", "")
	if err == nil {
		t.Error("Expected response to have an error")
	}
}

func TestFollowRealmGoodAuth(t *testing.T) {
	msg := "realm"
	resp, err := doreq("GET", ts.URL+"/realm", nil, "", realmUser.user+":"+realmUser.pass, "")
	if err != nil {
		t.Error("Expected response to have no error")
	}
	if resp.StatusCode != 200 {
		t.Error("Expected response status code to be 200 instead of ", resp.StatusCode)
	}
	body := getBody(resp)
	if body != msg {
		t.Error("Expected body to be '" + msg + "' instead of '" + body + "'")
	}
}

/*
 * test command-line parsing
 */
func TestSingleHeader(t *testing.T) {

}
func TestMultipleHeader(t *testing.T) {

}

/*
 * test output
 */
func TestHeadersOnly(t *testing.T) {
	setPrintHeaders(true)
	setPrintBody(false)
}
func TestHeadersAndBody(t *testing.T) {
	setPrintHeaders(true)
	setPrintBody(true)
}
func TestBodyOnly(t *testing.T) {
	setPrintHeaders(false)
	setPrintBody(true)
}

/*
 * Supporting func
 */

func getBody(resp *http.Response) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String()
}
