// groet project groet.go
package groet

import (
	"net/http"
	"regexp"
	"strings"
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
	match       string                                   // Any of the matching values for paths, exactpaths, domains, hosts, ports, methods, protocols or matchPaths
	matchFunc   func(*http.Request, string) bool         // A function to use to determine if this entry must be used for routing
	handler     http.Handler                             // A handler interface for this entry to use to handle ServeHTTP
	handlerFunc func(http.ResponseWriter, *http.Request) // A specific func to use to handle the serving of HTTP
	subRouter   *Router                                  // A sub router to use
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
	tmp := &RouterEntry{match: pth}
	rt.paths[pth] = tmp
	return tmp
}

// PathExact is for matching the full path
// This is normally used at the root level
func (rt *Router) PathExact(pth string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{match: pth}
	rt.exactPaths[pth] = tmp
	return tmp
}

// Domain is for matching a specific domain within the request URL
func (rt *Router) Domain(dom string) *RouterEntry {
	if rt == nil {
		return nil
	}
	dom = strings.ToLower(dom)
	tmp := &RouterEntry{match: dom}
	rt.paths[dom] = tmp
	return tmp
}

// Host matches a specific hostname (ignoring the domain)
func (rt *Router) Host(host string) *RouterEntry {
	if rt == nil {
		return nil
	}
	host = strings.ToLower(host)
	tmp := &RouterEntry{match: host}
	rt.paths[host] = tmp
	return tmp
}

// Match will only map if the current path element is matching the pattern provided
func (rt *Router) Match(mtch string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{match: mtch}
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
	tmp := &RouterEntry{match: prt}
	rt.ports[prt] = tmp
	return tmp
}

// Method will only match the provided method
func (rt *Router) Method(mthd string) *RouterEntry {
	if rt == nil {
		return nil
	}
	mthd = strings.ToUpper(mthd)
	tmp := &RouterEntry{match: mthd}
	rt.methods[mthd] = tmp
	return tmp
}

// Protocol will match either http or https as provided in prt
func (rt *Router) Protocol(prt string) *RouterEntry {
	if rt == nil {
		return nil
	}
	tmp := &RouterEntry{match: prt}
	rt.protocols[prt] = tmp
	return tmp
}

// getHostParts take the request and return the host,domain,port
func getHostParts(req *http.Request) (string, string, string) {
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
		_, _, port := getHostParts(req)
		rte, ok := rt.ports[port]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.domains) > 0 {
		_, domain, _ := getHostParts(req)
		rte, ok := rt.domains[domain]
		if ok {
			rte.ServeHTTP(rw, req)
			return
		}
	}
	if len(rt.hosts) > 0 {
		host, _, _ := getHostParts(req)
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
	pth := tmp[0]     // The current path element is the first one in the path slice
	if len(tmp) > 0 { // If there is still elements left in the path
		req.Header["PATH"] = tmp[1:]                                 // Save path elements that is still eleft
		req.Header["FULLPATH"] = append(req.Header["FULLPATH"], pth) // Build the full path that we've mapped so far
	}
	if len(pth) > 0 { // If there is actually still a path element to work with
		if len(rt.paths) > 0 { // Check for the path element
			rte, ok := rt.paths[pth] // Check if the current path element is in rt.paths
			if ok {
				rte.ServeHTTP(rw, req)
				return
			}
		}

		if len(rt.matchPaths) > 0 { // If there are regexp entries
			for _, rte := range rt.matchPaths { // Go through all the regexp entries
				if mtch, _ := regexp.MatchString(rte.match, pth); mtch { // If an entry matches then use it
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
	rte.handlerFunc = hnd
}

// ServeHTTP will handle the actual request for the entry
func (rte *RouterEntry) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if rte.handlerFunc != nil { // If there is a handler function then use it
		rte.handlerFunc(rw, req)
	} else if rte.handler != nil { // If there is a handler interface entry then use it
		rte.handler.ServeHTTP(rw, req)
	} else if rte.subRouter != nil { // If there is a sub router then tell it to handle the request
		rte.subRouter.ServeHTTP(rw, req)
	} else {
		http.NotFoundHandler().ServeHTTP(rw, req) // If nothing then let NotFoundHandler handle it
	}
}
