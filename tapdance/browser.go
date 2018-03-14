package tapdance

import (
	"github.com/wirepair/gcd"
	"github.com/wirepair/gcd/gcdapi"

	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

var ChromePath = ""
var conn map[*gcd.ChromeTarget]*TapdanceFlowConn
var overt map[*gcd.ChromeTarget]string
var current_url map[*gcd.ChromeTarget]string

func (flowConn *TapdanceFlowConn) Browse(overt_host string) {
	var err error

	if conn == nil {
		conn = make(map[*gcd.ChromeTarget]*TapdanceFlowConn)
		overt = make(map[*gcd.ChromeTarget]string)
		current_url = make(map[*gcd.ChromeTarget]string)
	}

	if flowConn.perDomainInflight == nil {
		flowConn.perDomainInflight = make(map[string]int)
	}

	tmpDir, err := ioutil.TempDir("/tmp/", "ditto-gcache")
	if err != nil {
		Logger().Errorln(err)
		return
	}

	debugger := gcd.NewChromeDebugger()
	debugger.AddFlags([]string{"--headless"})
	debugger.StartProcess(ChromePath, tmpDir, "9222")
	defer debugger.ExitProcess()

	target, err := debugger.NewTab()
	if err != nil {
		Logger().Warnln(flowConn.tdRaw.idStr()+" error getting targets: ", err)
	}

	conn[target] = flowConn
	overt[target] = overt_host

	target.Subscribe("Page.loadEventFired", loadEventFired)

	_, err = target.Page.Enable()
	if err != nil {
		Logger().Warnln(flowConn.tdRaw.idStr()+" error getting page: ", err)
	}

	target.Subscribe("Network.requestIntercepted", requestIntercepted)

	// TODO: Decide if requests made to non-overt hosts should be intercepted
	// TODO: Can this argument be an empty slice?
	_, err = target.Network.SetRequestInterception([]*gcdapi.NetworkRequestPattern{{}})
	if err != nil {
		Logger().Warnln(flowConn.tdRaw.idStr()+" error setting interception: ", err)
	}

	current_url[target] = "https://" + overt[target]
	Logger().Infoln(flowConn.tdRaw.idStr()+" navigating to ", current_url[target])

	_, _, _, err = target.Page.Navigate(current_url[target], "", "", "")
	if err != nil {
		Logger().Warnln(flowConn.tdRaw.idStr()+" error navigating to ", current_url[target], ": ", err)
	}

	select {}
}

func loadEventFired(target *gcd.ChromeTarget, event []byte) {
	var err error

	rand.Seed(time.Now().UnixNano())

	// TODO: Improve page stay heuristics
	duration := time.Duration(math.Abs(rand.NormFloat64()*2+5) * float64(time.Second))

	Logger().Infoln(conn[target].tdRaw.idStr()+" sleeping for ", int(duration/time.Second), " seconds...")

	time.Sleep(duration)

	dom := target.DOM
	root, err := dom.GetDocument(-1, true)
	if err != nil {
		Logger().Warnln(conn[target].tdRaw.idStr()+" error getting root: ", err)
	}

	links, err := dom.QuerySelectorAll(root.NodeId, "a")
	if err != nil {
		Logger().Warnln(conn[target].tdRaw.idStr()+" error getting links: ", err)
	}

	var candidate_links []string

	for _, l := range links {
		attributes, err := dom.GetAttributes(l)
		if err != nil {
			Logger().Warnln(conn[target].tdRaw.idStr()+" error getting attributes: ", err)
		}

		attributesMap := make(map[string]string)
		for i := 0; i < len(attributes); i += 2 {
			attributesMap[attributes[i]] = attributes[i+1]
		}

		if _, hasHref := attributesMap["href"]; !hasHref {
			continue
		}

		candidateUrl, err := url.Parse(attributesMap["href"])
		if err != nil {
			fmt.Printf("Could not parse url %s: %v\n", attributesMap["href"], err)
			continue
		}
		currUrl, err := url.Parse(current_url[target])
		if err != nil {
			fmt.Printf("Could not parse url %s: %v\n", attributesMap["href"], err)
			continue
		}
		if candidateUrl.Hostname() == "" {
			candidateUrl = currUrl.ResolveReference(candidateUrl)
			candidateUrl.Host = currUrl.Hostname()
		}
		candidateUrl.Fragment = ""
		if candidateUrl.Hostname() != currUrl.Hostname() || candidateUrl.Hostname() == "" {
			// check whether link is internal
			fmt.Printf("link %s is not internal. overt url: %s\n",
				candidateUrl.Hostname(), currUrl.Hostname())
			continue
		}

		if !isLeaf(candidateUrl.String()) {
			candidate_links = append(candidate_links, candidateUrl.String())
		}
	}

	if len(candidate_links) > 0 {
		// TODO: Handle broken links
		prev_curr_url := current_url[target]
		current_url[target] = candidate_links[rand.Intn(len(candidate_links))]
		Logger().Infoln(conn[target].tdRaw.idStr()+" following link to ", current_url[target])

		_, _, _, err = target.Page.Navigate(current_url[target], prev_curr_url, "", "")
		if err != nil {
			Logger().Warnln(conn[target].tdRaw.idStr()+" error navigating to ", current_url[target], ": ", err)
		}
	} else {
		// Improve out-of-link handling, possibly try returning to previous page
		Logger().Infoln(conn[target].tdRaw.idStr() + " ran out of links, refreshing current page")

		_, _, _, err = target.Page.Navigate(current_url[target], current_url[target], "", "")
		if err != nil {
			Logger().Warnln(conn[target].tdRaw.idStr()+" error navigating to ", current_url[target], ": ", err)
		}
	}
}

var leafExtensions = []string{
	".jpg",
	".jpeg",
	".png",
	".gif",
	".svg",
	".ico",
	".dat",
}

func isLeaf(targetUrl string) bool {
	extension := strings.ToLower(filepath.Ext(targetUrl))
	for _, le := range leafExtensions {
		if extension == le {
			return true
		}
	}
	return false
}

func requestIntercepted(target *gcd.ChromeTarget, event []byte) {
	var err error

	eventUnmarshal := &gcdapi.NetworkRequestInterceptedEvent{}
	err = json.Unmarshal(event, eventUnmarshal)
	if err != nil {
		Logger().Warnln(conn[target].tdRaw.idStr()+" error unmarshalling request: ", err)
	}

	Logger().Infoln(conn[target].tdRaw.idStr()+" intercepted request to: ", eventUnmarshal.Params.Request.Url)

	request, err := http.NewRequest(eventUnmarshal.Params.Request.Method, eventUnmarshal.Params.Request.Url, strings.NewReader(eventUnmarshal.Params.Request.PostData))
	if err != nil {
		Logger().Warnln(conn[target].tdRaw.idStr()+" error creating request: ", err)
	}

	for k, v := range eventUnmarshal.Params.Request.Headers {
		request.Header.Set(k, v.(string))
	}

	// Doesn't seem to be required
	//if request.Header.Get("Host") == "" {
	//	request.Header.Set("Host", request.URL.Host)
	//}

	// DevTools doesn't supply these headers; find way to get rid of hardcode
	if request.Header.Get("Connection") == "" {
		request.Header.Set("Connection", "keep-alive")
	}

	if request.Header.Get("Accept-Encoding") == "" {
		request.Header.Set("Accept-Encoding", "                 ") // TODO: Handle "gzip, deflate" or "gzip, deflate, br"?
	}

	// Discrepancies between GUI and headless Chrome; find a better fix
	if ua := request.Header.Get("User-Agent"); ua != "" {
		request.Header.Set("User-Agent", strings.Replace(ua, "HeadlessChrome", "Chrome", -1))
	}
	if request.Header.Get("Accept-Language") == "" {
		request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}

	var direct bool

	var direct_response *http.Response
	var response string

	// TODO: Investigate failed binary file fetches
	for {
		for {
			conn[target].browserConnPoolMutex.Lock()

			if !conn[target].resourceRequestInflight {
				direct = false

				conn[target].resourceRequestInflight = true
				val, _ := conn[target].perDomainInflight[request.URL.Host]
				conn[target].perDomainInflight[request.URL.Host] = val + 1

				conn[target].browserConnPoolMutex.Unlock()
				break
			}

			if conn[target].directRequestInflight < 9 {
				if val, _ := conn[target].perDomainInflight[request.URL.Host]; val < 6 {
					direct = true

					conn[target].directRequestInflight += 1
					val, _ := conn[target].perDomainInflight[request.URL.Host]
					conn[target].perDomainInflight[request.URL.Host] = val + 1

					conn[target].browserConnPoolMutex.Unlock()
					break
				}
			}

			conn[target].browserConnPoolMutex.Unlock()
		}

		if direct {
			Logger().Infoln(conn[target].tdRaw.idStr()+" firing direct request to: ", eventUnmarshal.Params.Request.Url)
			direct_response, err = conn[target].directRequestClient.Do(request)
		} else {
			Logger().Infoln(conn[target].tdRaw.idStr()+" firing resource request to: ", eventUnmarshal.Params.Request.Url)
			response, err = conn[target].resourceRequest(request)
		}

		conn[target].browserConnPoolMutex.Lock()

		if direct {
			conn[target].directRequestInflight -= 1
			conn[target].perDomainInflight[request.URL.Host] -= 1
		} else {
			conn[target].resourceRequestInflight = false
			conn[target].perDomainInflight[request.URL.Host] -= 1
		}

		conn[target].browserConnPoolMutex.Unlock()

		if err != nil {
			Logger().Warnln(conn[target].tdRaw.idStr()+" failed to fetch ", eventUnmarshal.Params.Request.Url, ", retrying... (", err, ")")
		} else {
			break
		}
	}

	if direct {
		headers := new(bytes.Buffer)
		err = direct_response.Header.Write(headers)
		if err != nil {
			Logger().Warnln(conn[target].tdRaw.idStr()+" error reading header: ", err)
		}

		defer direct_response.Body.Close()
		body := new(bytes.Buffer)
		_, err = body.ReadFrom(direct_response.Body)
		if err != nil {
			Logger().Warnln(conn[target].tdRaw.idStr()+" error reading body: ", err)
		}

		response = fmt.Sprintf("%s %s\r\n%s\r\n\r\n%s", direct_response.Proto, direct_response.Status, headers.String(), body.String())
	}

	_, err = target.Network.ContinueInterceptedRequest(eventUnmarshal.Params.InterceptionId, "", base64.StdEncoding.EncodeToString([]byte(response)), "", "", "", map[string]interface{}{}, nil)
	if err != nil {
		Logger().Warnln(conn[target].tdRaw.idStr()+" error replying to intercept: ", err)
	}
}
