# http

`http` is a subset of curl that does one thing well: **it follows Bearer authentication**.

If you need to make a request against something that users Bearer authentication, it works like this:

1. `GET http://actual.target.com/abc` 
2. Receive a `Www-Authenticate: Bearer realm=http://somewhere.else.com,service=a.b.c`
3. Make a new request (probably with different credentials) to `http://somewhere.else.com` with the right parameters and crendetials
4. Get back a token, usually a Json Web Token.
5. Make a new request to `GET http://actual.target.com/abc` with the token in the right header

The protocol is great... but testing against it is a pain.
Docker registry v2 works like this... and testing against it is a pain.

`http` comes to solve it.

````
http --realm-user jim:smith http://www.foo.com/abc
````

## Options
`http` supports a subset of curl options, although it is growing.

* `-d BODY` or `--data BODY`: send `BODY` as the body of the request
* `-X METHOD` or `--request METHOD`: use http method `METHOD`, defaults to `GET`
* `-u USER:PASS` or `--user USER:PASS`: use `USER` and `PASS` as credentials for http Basic authentication
* `--realm-user USER:PASS`: use `USER` and `PASS` as credentials for authentication against the token auth service
* `-H HEADER -H HEADER ... -H HEADER` or `--header HEADER --header HEADER ... --header HEADER`: use `HEADER` as headers for the http request. Use as many as you want.
* `-i` or `--include`: in addition to the body, print the http response headers
* `-I` or `--head`: print *just* the http response headers
* `-V` or `--verbose`: be verbose about what you are doing

## Installation
`http` is written in go, which means it is compiled locally for each platform (as of this writing, Linux and Mac). Just download and run

## Build
Built using go. Clone this repository and build. I used [gb](http://getgb.io) to build it.

## License
MIT
