// groet project groet.go
// Author: Hannes du Plooy
// Revision Date:
// Basic router library for golang
//
// 28 Aug 2016
//     Initial
// 9 Sep 2016
// 15 Sep 2016
//    Added HandleIf, HandleSelect, ServeFiles, ServeTemplate
//    Dropped match string in RouterEntry was only used by regexp matching and that is done now by matchFunc
//    Dropped handlerFunc etc they are handled by http.Handlers now
// 16 Sep 2016
//    Added Template serving as action
//    Added FileServing as action
//    Added Split (an if .. then .. else type decision to take one or other action)
//    Added Select (similar to split but a action from a slice of actions is selected)
// 19 Sep 2016
//    Fixed Domain() and Host() to add to domains and hosts individually
package groet

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// Router keep all the rules for routes
type Router struct {
	paths      map[string]*RouterEntry // Each part of a path can be evaluated one by one /path1/path2/path3 .......
	exactPaths map[string]*RouterEntry // This is normally used at the root level to map to a full exact path
	domains    map[string]*RouterEntry // Map to a specific domains (allow you to do virtual domains)
	hosts      map[string]*RouterEntry // Map to specific hosts (within a domain)
	ports      map[string]*RouterEntry // Map to different ports, that is if you are using multiple servers for different ports going to the same routers
	methods    map[string]*RouterEntry // Map to different methods like POST,GET,ADD,DELETE etc.
	protocols  map[string]*RouterEntry // Map to either http or https (for now)
	matchPaths []*RouterEntry          // Match if regular expression matches the current path element
	funcPaths  []*RouterEntry          // Match if func returns true - func(*http.Request,pth) bool
	anyPath    *RouterEntry            // If nothing else in the Router match then use this one
}

// RouterEntry is used for keeping the matching entries within a Router struct
// This will be created by one of the Router functions and then you can connect a handler, handler func or subrouter to it
type RouterEntry struct {
	matchFunc func(*http.Request, string) bool // A function to use to determine if this entry must be used for routing
	handler   http.Handler                     // A handler interface for this entry to use to handle ServeHTTP
	subRouter *Router                          // A sub router to use
}

// NewRouter creates a blank router that can then be populated
func NewRouter() *Router {
	return &Router{make(map[string]*RouterEntry), make(map[string]*RouterEntry), make(map[string]*RouterEntry), make(map[string]*RouterEntry), make(map[string]*RouterEntry), make(map[string]*RouterEntry), make(map[string]*RouterEntry), make([]*RouterEntry, 0, 5), make([]*RouterEntry, 0, 5), nil}
}

// Path will add a new path element checker to a Route
// You can have a Path that points to a sub router that then points to further paths
// For example /path1/path2, /path1/path3 etc.
func (rt *Router) Path(pth string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{}
	rt.paths[pth] = tmp
	return tmp
}

// PathExact is for matching the full path
// This is normally used at the root level
func (rt *Router) PathExact(pth string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{}
	rt.exactPaths[pth] = tmp
	return tmp
}

// Domain is for matching a specific domain within the request URL
func (rt *Router) Domain(dom string) *RouterEntry {
	if rt == nil {
		return nil
	}
	dom = strings.ToLower(dom)
	tmp := &RouterEntry{}
	rt.domains[dom] = tmp
	return tmp
}

// Host matches a specific hostname (ignoring the domain)
func (rt *Router) Host(host string) *RouterEntry {
	if rt == nil {
		return nil
	}
	host = strings.ToLower(host)
	tmp := &RouterEntry{}
	rt.hosts[host] = tmp
	return tmp
}

// Match will only map if the current path element is matching the pattern provided
func (rt *Router) Match(match string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{matchFunc: func(r *http.Request, pth string) bool {
		mtch, _ := regexp.MatchString(match, pth)
		return mtch
	}}
	rt.matchPaths = append(rt.matchPaths, tmp)
	return tmp
}

// Func provide a function that when called and returns a true then it will be used to handle the request
func (rt *Router) Func(f func(*http.Request, string) bool) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{matchFunc: f}
	rt.funcPaths = append(rt.funcPaths, tmp)
	return tmp
}

// Any is a catch all when nothing else matches
func (rt *Router) Any() *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{}
	rt.anyPath = tmp
	return tmp
}

// Port will match only the specified prot
func (rt *Router) Port(prt string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{}
	rt.ports[prt] = tmp
	return tmp
}

// Method will only match the provided method
func (rt *Router) Method(mthd string) *RouterEntry {
	if rt == nil {
		return nil
	}
	mthd = strings.ToUpper(mthd)
	tmp := &RouterEntry{}
	rt.methods[mthd] = tmp
	return tmp
}

// Protocol will match either http or https as provided in prt
func (rt *Router) Protocol(prt string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{}
	rt.protocols[prt] = tmp
	return tmp
}

// GetHostParts take the request and return the host,domain,port
func GetHostParts(req *http.Request) (string, string, string) {
	host := req.Host
	pos := strings.Index(host, ":")
	domain := ""
	port := "80"
	if pos > 0 {
		port = host[pos+1:]
		host = host[:pos]
	}
	pos = strings.Index(host, ".")
	if pos > 0 {
		domain = host[pos+1:]
		host = host[:pos]
	}
	return host, domain, port
}

// GetHost - utility function to get the host part of the call from the client
func GetHost(req *http.Request) string {
	host, _, _ := GetHostParts(req)
	return host
}

// GetDomain - utility function to get the domain part of the call from the client
func GetDomain(req *http.Request) string {
	_, dom, _ := GetHostParts(req)
	return dom
}

// GetPort - utility function to get the port part of the call from the client
func GetPort(req *http.Request) string {
	_, _, port := GetHostParts(req)
	return port
}

// Router.ServeHTTP will determine which of the entries match and then use that to process the request
// The order in which the testing is done is protocols, methods, ports, domains,hosts, exact paths,
//    normal paths, regexp matches, function paths or any catch alls
func (rt *Router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if len(rt.protocols) > 0 {
		prot := "http"
		// If there is TLS info then the protocol is https
		if req.TLS != nil {
			prot = "https"
		}
		rte, ok := rt.protocols[prot]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.methods) > 0 {
		rte, ok := rt.methods[req.Method]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.ports) > 0 {
		_, _, port := GetHostParts(req)
		rte, ok := rt.ports[port]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.domains) > 0 {
		_, domain, _ := GetHostParts(req)
		rte, ok := rt.domains[domain]
		fmt.Printf("dom=%s ok=%t\n ", domain, ok)
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.hosts) > 0 {
		host, _, _ := GetHostParts(req)
		rte, ok := rt.hosts[host]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.exactPaths) > 0 {
		rte, ok := rt.exactPaths[req.URL.Path]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	// We're making use of PATH as a request header
	// It holds the current path elements as it is being processed by the subrouters
	// If the url has the following path /path1/testa/.....
	//  this will be kept in the PATH header as  []string{"path1","testa",....}
	//  with each subrouter and path match the PATH will shift one to the right
	//  for checking at the next level
	tmp, ok := req.Header["PATH"]
	if !ok {
		// If there is no PATH element in req.Header then it is the first time we encounter a path matcher
		// So split based on / and ignore the leading empty string in the returned slice
		tmp = strings.Split(req.URL.Path, "/")[1:]
	}
	pth := ""
	if len(tmp) > 0 {
		pth = tmp[0] // The current path element is the first one in the path slice
	}
	if len(tmp) > 0 { // If there is still elements left in the path
		req.Header["PATH"] = tmp[1:]                                 // Save path elements that is still eleft
		req.Header["FULLPATH"] = append(req.Header["FULLPATH"], pth) // Build the full path that we've mapped so far
	} else {
		req.Header["PATH"] = make([]string, 0)
	}
	if len(pth) >= 0 { // If there is actually still a path element to work with
		if len(rt.paths) > 0 { // Check for the path element
			rte, ok := rt.paths[pth] // Check if the current path element is in rt.paths
			if ok {
				rte.ServeHTTP(rw, req)
				return
			}
		}

		if len(rt.matchPaths) > 0 { // If there are regexp entries
			for _, rte := range rt.matchPaths { // Go through all the regexp entries
				if rte.matchFunc(req, pth) {
					rte.ServeHTTP(rw, req)
					return
				}
			}
		}
	}
	if len(rt.funcPaths) > 0 { // If there are function entries for this router
		for _, rte := range rt.funcPaths { // Go through all function entries
			if rte.matchFunc != nil && rte.matchFunc(req, pth) { // If the func is not nil and it returns a true then handle request
				rte.ServeHTTP(rw, req)
				return
			}
		}
	}

	if rt.anyPath != nil { // If nothing matched so far and the anyPath entry exists then handle it else NotFoundHandler is called
		rt.anyPath.ServeHTTP(rw, req)
	} else {
		http.NotFoundHandler().ServeHTTP(rw, req)
	}
}

// Subrouter assings a subrouter to a RouterEntry
// This allows for a hierarchical routers to handle sub cases
func (rte *RouterEntry) Subrouter(rt2 *Router) {
	if rte == nil {
		return
	}
	rte.subRouter = rt2
}

// FuncHandler is a struct that is a http.Handler for our own funcs
type FuncHandler struct {
	FN func(http.ResponseWriter, *http.Request)
}

func (fh *FuncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fh.FN(w, r)
}

// Handle assigns a http.Handler interface based value to handle requests
func (rte *RouterEntry) Handle(hnd http.Handler) {
	if rte == nil {
		return
	}
	rte.handler = hnd
}

// HandleFunc does exactly that, it will assign the function to use to handle requests for the entry
func (rte *RouterEntry) HandleFunc(hnd func(http.ResponseWriter, *http.Request)) {
	if rte == nil {
		return
	}
	rte.handler = &FuncHandler{hnd}
}

// HandleSplit will execute func hnd if a true is returned the thenpart handler is processed else the elsepart handler
// This allows us to do basic decisions when routes are declared and then to take either one path or another
func (rte *RouterEntry) HandleSplit(hnd func(*http.Request) bool, thenpart http.Handler, elsepart http.Handler) {
	if rte == nil {
		return
	}
	rte.handler = &FuncHandler{func(w http.ResponseWriter, r *http.Request) {
		if hnd(r) {
			thenpart.ServeHTTP(w, r)
		} else {
			elsepart.ServeHTTP(w, r)
		}
	}}
}

// HandleSelect will execute hnd which returns an int which is used to select the handler in actions which will handle the action further
// else NotFoundHandler is asked to process the action. So the best is to make sure the hnd function returns a valid int
func (rte *RouterEntry) HandleSelect(hnd func(*http.Request) int, actions ...http.Handler) {
	if rte == nil {
		return
	}
	rte.handler = &FuncHandler{func(w http.ResponseWriter, r *http.Request) {
		pos := hnd(r)
		if pos < 0 || pos >= len(actions) {
			http.NotFoundHandler().ServeHTTP(w, r) // If nothing then let NotFoundHandler handle it
		} else {
			actions[pos].ServeHTTP(w, r)
		}
	}}
}

// ServeFiles is a basic file server a the specified path.
func (rte *RouterEntry) ServeFiles(path string, defexts []string) {
	if rte == nil {
		return
	}
	if !strings.HasSuffix(path, string(os.PathSeparator)) {
		path += string(os.PathSeparator)
	}
	rte.handler = &FuncHandler{func(w http.ResponseWriter, r *http.Request) {
		fname := path + strings.Join(r.Header["PATH"], string(os.PathSeparator))
		st, err := os.Stat(fname)
		if err != nil { // If it is not a file or directory, then not found
			http.NotFoundHandler().ServeHTTP(w, r)
		} else {
			if st.IsDir() { // If it is a directory search for and index file
				if fname[len(fname)-1] != '/' { // Add a trailing / if none
					fname += "/"
				}
				// If index.html exists within the directory then serve it
				if st, err = os.Stat(fname + "index.html"); err == nil && !st.IsDir() {
					http.ServeFile(w, r, fname+"index.html")
				} else {
					// For every cgi extension see if that index file exists
					//   if it does then let pass it on to the cgi handler
					for _, ext := range defexts {
						if st, err = os.Stat(fname + "index." + ext); err == nil && !st.IsDir() {
							http.ServeFile(w, r, fname+"index."+ext)
							return
						}
					}
					// If nothing matches then do a not found (we don't do directory listings)
					http.NotFoundHandler().ServeHTTP(w, r)
				}
			} else {
				// If it is a file then serve it
				http.ServeFile(w, r, fname)
			}
		}
	}}
}

// ServeTemplate will call the func f which returns the template name to use and the data for it
// temp is then asked to execute the correct template
func (rte *RouterEntry) ServeTemplate(f func(*http.Request) (string, interface{}), temp *template.Template) {
	if rte == nil {
		return
	}
	rte.handler = &FuncHandler{func(w http.ResponseWriter, r *http.Request) {
		nm, data := f(r)
		temp.ExecuteTemplate(w, nm, data)
	}}
}

// TemplateHandleFunc is a func that can be called anywhere with func f providing the template name and the data
// temp is then asked to execute the correct template
func TemplateHandleFunc(f func(*http.Request) (string, interface{}), temp *template.Template) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		nm, data := f(r)

		temp.ExecuteTemplate(w, nm, data)
	}
}

// ServeHTTP will handle the actual request for the entry
func (rte *RouterEntry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rte.handler != nil { // If there is a handler interface entry then use it
		rte.handler.ServeHTTP(w, r)
	} else if rte.subRouter != nil { // If there is a sub router then tell it to handle the request
		rte.subRouter.ServeHTTP(w, r)
	} else {
		http.NotFoundHandler().ServeHTTP(w, r) // If nothing then let NotFoundHandler handle it
	}
}

// ParseTemplates is an utility function to walk from a path and then deeper and to
// add them all to a template and return that template
// ext is the extension of the template files to look for
func ParseTemplates(path string, ext string) *template.Template {
	temp := &template.Template{}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ext) {
			_, err := temp.ParseFiles(path)
			if err != nil {
				log.Printf("Error with parsing of templates: %s\n", err.Error())
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error with parsing of templates: %s\n", err.Error())
		return nil
	}
	return temp
}
