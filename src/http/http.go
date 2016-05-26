/*
 * do http requests, but allow following of bearer authentication
 * main motivation is to enable simple testing of docker v2 registry
 */

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/urfave/cli"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type UserAuthStruct struct {
	user string
	pass string
}
type RealmAuthStruct struct {
	realm  string
	params map[string]string
}

var verbose bool
var printHeaders bool = false
var printBody bool = true

func main() {
	// get my command-line args
	app := cli.NewApp()
	app.Name = "http"
	app.Usage = "transfer a URL with realm auth support"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "request, X",
			Value: "GET",
			Usage: "http `METHOD`",
		},
		cli.StringFlag{
			Name:  "data, d",
			Value: "",
			Usage: "request body `DATA`",
		},
		cli.StringFlag{
			Name:  "user, u",
			Value: "",
			Usage: "authentication credentials as `username:password`",
		},
		cli.StringFlag{
			Name:  "realm-user",
			Value: "",
			Usage: "authentication credentials for realm provided in www-authenticate as `username:password`",
		},
		cli.StringSliceFlag{
			Name:  "header, H",
			Value: &cli.StringSlice{},
			Usage: "add `HEADER` to request, use multiple times",
		},
		cli.BoolFlag{
			Name:  "include, i",
			Usage: "show headers",
		},
		cli.BoolFlag{
			Name:  "head, I",
			Usage: "show headers only",
		},
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "be verbose about what is going on",
		},
	}
	app.Action = func(c *cli.Context) error {
		// did we ask for verbose?
		verbose = c.Bool("V")

		method := c.String("X")
		body := c.String("d")
		auth := c.String("u")
		realmAuth := c.String("realm-user")
		showHeaders := c.Bool("i")
		headersOnly := c.Bool("I")
		headers := c.StringSlice("H")

		setPrintHeaders(showHeaders || headersOnly)
		setPrintBody(!headersOnly)

		// get the url
		allurls := c.Args()
		if len(allurls) < 1 {
			cli.ShowAppHelp(c)
			return nil
		}
		oneurl, err := cleanUrl(allurls[0])

		resp, err := doreq(method, oneurl, headers, auth, realmAuth, body)

		if err != nil {
			fmt.Println(err)
			return cli.NewExitError("Failed request", 1)
		}
		// show the response
		printOutput(resp)
		return nil
	}

	app.Run(os.Args)
}

func setPrintBody(print bool) {
	printBody = print
}
func setPrintHeaders(print bool) {
	printHeaders = print
}

func log(msg string) {
	if verbose {
		fmt.Println(msg)
	}
}
func logMsg(msg string) {
	log("* " + msg)
}
func logIn(msg string) {
	log("< " + msg)
}
func logOut(msg string) {
	log("> " + msg)
}

func printOutput(resp *http.Response) {
	// show the response code
	// show the headers if requests
	if printHeaders {
		dump, err := httputil.DumpResponse(resp, false)
		_ = err
		fmt.Printf("%s\n", dump)
	}
	if printBody {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		fmt.Print(buf.String())
	}
}

func cleanUrl(dirty string) (clean string, err error) {
	u, err := url.Parse(dirty)

	// was there any problem with it?
	if err == nil {
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		clean = u.String()
	}
	return clean, err
}

func doreq(method string, url string, headers []string, auth string, realmAuth string, body string) (resp *http.Response, err error) {
	bodyStr := strings.NewReader(body)
	request, err := http.NewRequest(method, url, bodyStr)

	// did we have auth creds?
	authStruct, err := getAuthStruct(auth)
	if err == nil {
		request.SetBasicAuth(authStruct.user, authStruct.pass)
	}

	// check err
	client := &http.Client{}
	resp, err = client.Do(request)

	// did we have a 401 with www-authenticate?
	if err == nil && resp.StatusCode == 401 && realmAuth != "" {
		authCmd := resp.Header.Get("www-authenticate")
		if authCmd != "" && strings.HasPrefix(authCmd, "Bearer") {
			authParts := strings.Fields(authCmd)
			return doRealmReq(method, url, body, auth, realmAuth, authParts[1])
		}
	}

	return resp, err
}

func doRealmReq(method string, url string, body string, auth string, realmAuth string, authCmd string) (resp *http.Response, err error) {
	realmAuthStruct := objectify(authCmd)
	logMsg("Following realm auth to: " + realmAuthStruct.realm)

	// did we have auth creds?
	userAuthStruct, err := getAuthStruct(realmAuth)
	token, err := authenticateBearer(userAuthStruct, realmAuthStruct)

	if err != nil {
		return nil, err
	}
	logMsg("Token " + token)

	// redo the request, setting the token as a header to:
	bodyStr := strings.NewReader(body)
	request, err := http.NewRequest(method, url, bodyStr)
	request.Header.Set("Authorization", "Bearer "+token)

	// check err
	client := &http.Client{}
	return client.Do(request)
}

func authenticateBearer(creds UserAuthStruct, realmAuth RealmAuthStruct) (string, error) {
	// add params from realmAuth.params as query properties
	u, err := url.Parse(realmAuth.realm)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for key, val := range realmAuth.params {
		q.Set(key, val)
	}
	u.RawQuery = q.Encode()
	request, err := http.NewRequest("GET", u.String(), nil)
	// did we have auth creds?
	if err == nil {
		request.SetBasicAuth(creds.user, creds.pass)
	}

	// check err
	client := &http.Client{}
	resp, err := client.Do(request)

	// did we have a valid response?
	if err != nil {
		return "", err
	} else if resp.StatusCode != 200 {
		// if we did not get a token, we need to dump the output
		logMsg("Failed to get token ")
		printOutput(resp)
		return "", errors.New("Failed to get token from " + realmAuth.realm)
	} else {
		var data map[string]interface{}
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&data)
		if err == nil {
			return data["token"].(string), nil
		} else {
			return "", err
		}
	}
}

func getAuthStruct(auth string) (UserAuthStruct, error) {
	if auth == "" {
		return UserAuthStruct{}, errors.New("invalid auth")
	} else {
		authA := strings.SplitN(auth, ":", 2)
		user, pass := authA[0], ""
		if len(authA) > 1 {
			pass = authA[1]
		}
		return UserAuthStruct{user: user, pass: pass}, nil
	}
}

func objectify(input string) RealmAuthStruct {
	ras := RealmAuthStruct{params: make(map[string]string)}
	// first split to get the sections
	authParts := strings.Split(input, ",")
	// now with each one, get a=b
	for _, param := range authParts {
		keyval := strings.SplitN(param, "=", 2)
		// it is a string, so make sure to replace first and last " char if it exists
		param := strings.TrimPrefix(keyval[1], "\"")
		param = strings.TrimSuffix(param, "\"")
		if keyval[0] == "realm" {
			ras.realm = param
		} else {
			ras.params[keyval[0]] = param
		}
	}
	return ras
}
