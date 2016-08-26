# hduplooy/groet 

## Straight forward HTTP Router

The idea with was to have a simple Router not as big as gorilla/mux for example. So if anyone wants to do some serious regular expression matching etc. against anything in the path etc. rather use something like gorilla/mux.

With *groet* you create a new router then add entries to it to match various aspects of a http request, based on that a corresponding handler is called or a subrouter that can then handle more aspects of the request.

Here is an example:

    package main

    import (
	    "fmt"
	    "net/http"
	    "strings"

	    "github.com/hduplooy/groet"
    )

    type MyString string

    func (str MyString) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	    fmt.Fprintf(w, "<html><body><h1>Hello %s</h1></body></html>", str)
    }

    func Hello2(w http.ResponseWriter, r *http.Request) {
	    fmt.Fprintf(w, "<html><body><h1>Hello B</h1></body></html>")
    }

    func Hello4(w http.ResponseWriter, r *http.Request) {
	    fmt.Fprintf(w, "<html><body><h1>Hello C</h1><h2>%s</h2></body></html>", r.URL.Path)
    }

    func main() {
	    rt := groet.NewRouter()
	    rt2 := groet.NewRouter()
	    rt.Path("testa").Subrouter(rt2)
	    rt2.Path("alpha").Handle(MyString("Alpha"))
	    rt2.Path("beta").Handle(MyString("Beta"))
	    rt2.Match("tester*").HandleFunc(Hello4)
	    rt2.Func(func(r *http.Request, pth string) bool {
		    add := r.RemoteAddr
		    add = add[:strings.LastIndex(add, ":")]
		    fmt.Println(add)
		    return add == "127.0.0.1" || add == "[::1]"
	    }).HandleFunc(Hello4)
	    rt2.Method("POST").Handle(MyString("Poster"))
	    rt.Path("testb").HandleFunc(Hello2)
	    http.ListenAndServe(":8080", rt)
    }

`The following image shows how these routes fit together:
![](http://www.duplooy.org/groet.svg)

Thus the following will be handled:
<table>
<tr><td><bold>URL</bold></td><td><bold>Handling</bold></td></tr>
<tr><td>/testa/alpha/somemore</td><td>MyString("Alpha")</td></tr>
<tr><td>/test/alpha</td><td>MyString("Alpha")</td></tr>
<tr><td>/test/beta/rest</td><td>MyString("Beta")</td></tr>
<tr><td>/test/testerstuff/andmore</td><td>Hello4</td></tr>
<tr><td>/testb/otherstuff</td><td>Hello2</td></tr>
</table>
Any localhost requests will be matched by the anonymous function and passed on to Hello4 to handle and any posts are handled by MyString("Poster"). Both these will depend on the first /testa match for the root router.
