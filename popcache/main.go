package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/httprps"
	"github.com/yzp0n/ncdn/types"
)

var originURLStr = flag.String("originURL", "http://localhost:8888", "Origin server URL")
var listenAddr = flag.String("listenAddr", ":8889", "Address to listen on")
var nodeId = flag.String("nodeId", "unknown_node", "Name of the node")

type CacheHandler struct {
	proxy http.Handler
	cache map[string]CacheEntry
	mu    sync.RWMutex
}

type CacheEntry struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	StoredAt   time.Time
}

func (h *CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Host + r.URL.RequestURI()
	h.mu.RLock()
	val, ok := h.cache[key]
	h.mu.RUnlock()
	if ok {
		w.WriteHeader(val.StatusCode)
		w.Write(val.Body)
		return
	}

	h.proxy.ServeHTTP(w, r)
}

func (h *CacheHandler) modifier(res *http.Response) error {
	key := res.Request.Header.Get("X-Forwarded-Host") + res.Request.URL.RequestURI()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	_ = res.Body.Close()

	res.Body = io.NopCloser(bytes.NewReader(body))

	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache[key] = CacheEntry{
		StatusCode: res.StatusCode,
		Header:     res.Header,
		Body:       body,
		StoredAt:   time.Now(),
	}
	return nil
}

func main() {
	flag.Parse()

	originURL, err := url.Parse(*originURLStr)
	if err != nil {
		log.Fatalf("Failed to parse origin URL %q: %v", *originURLStr, err)
	}

	start := time.Now()

	mux := http.NewServeMux()
	rps := httprps.NewMiddleware(mux)
	http.Handle("/", rps)

	mux.HandleFunc("/statusz", func(w http.ResponseWriter, r *http.Request) {
		s := types.PoPStatus{
			Id:     *nodeId,
			Uptime: time.Since(start).Seconds(),
			Load:   rps.GetRPS(),
		}
		bs, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal PoP status: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write(bs)
	})
	mux.HandleFunc("/latencyz", func(w http.ResponseWriter, r *http.Request) {
		// return 204
		w.WriteHeader(http.StatusNoContent)
	})
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetXForwarded()
			r.Out.Header.Set("X-NCDN-PoPCache-NodeId", *nodeId)
			r.SetURL(originURL)
		},
	}

	cachehandler := &CacheHandler{
		proxy: proxy,
		cache: map[string]CacheEntry{},
		mu:    sync.RWMutex{},
	}
	proxy.ModifyResponse = cachehandler.modifier
	mux.Handle("/", cachehandler)

	log.Printf("Listening on %s...", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
