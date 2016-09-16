**Busy updating at this moment with templates, file serving, Splitting (if then), Selecting (same as switch). Will be up in about 15 minutes**

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

## Matchers
These are matchers on a router, if it is matched then its action is used.
* Path(pth string) - will match the current position in the url path to pth
* PathExact(pth string) - will match the full path of the url to pth
* Domain(dom string) - will match the domain part of the url to dom
* Host(host string) - will match the host part of the url to host
* Match(match string) - will do a regexp match of the current element in the path to match
* Func(f func(*http.Request,string) bool) - will execute the function f with the request and current path to determine if this route should be followed
* Any - is a catch all, if none of the others match then this will definitely match
* Port(prt string) - match the port part of the url to prt
* Method(mthd string) - will match the method (post,get,...) to mthd
* Protocol(prt string) - currently check if http or https

## Actions
These are the actions that can be handled for a router match.
* Subrouter(rt2 *Router) - pass processing over to rt2
* Handle(hnd http.Handler) - similar to the normal http.Handle
* HandleFunc(hnd func(http.ResponseWriter,*http.Request)) - similar to the normal http.HandleFunc
* HandleSplit(hnd,thenpart,elsepart) - call hnd with current request and if it returns true do thenpart else elspart
* HandleSelect(hnd,actions...) - cal hnd and based on the int returned select the action from actions to handle the request
* ServeFiles(path,defexts) - serve anything further as file that resides in path (defexts are for folders to get the default.html or whatever)
* ServeTemplate(f,temp) - call f which returns a template name and the data and then serve the appropriate template in temp

## Utility functions
* GetHostParts(req) - extract the host, domain and port from the request
* GetHost(req) - returns only the host part of the request
* GetDomain(req) - returns only the domain part of the request
* GetPort(req) - return only the port part of the request
* ParseTemplates(path,ext) - walk through all files at path and parse those with extension ext as templates and then return resulting template.Template

