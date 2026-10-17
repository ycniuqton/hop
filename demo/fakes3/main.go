// Command fakes3 is an in-memory S3 server for recording the README demos; never use it for real data.
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// conditionalWrites adds S3's If-None-Match: * on PUT, which hop needs and gofakes3 lacks, and
// denies ListBuckets like keys scoped to one bucket (so init shows no "too broad" warning).
type conditionalWrites struct {
	next    http.Handler
	backend *s3mem.Backend
}

func (c conditionalWrites) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/" {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<Error><Code>AccessDenied</Code><Message>scoped key</Message></Error>`))
		return
	}
	if r.Method == http.MethodPut && r.Header.Get("If-None-Match") == "*" {
		bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if _, err := c.backend.HeadObject(bucket, key); err == nil {
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`<Error><Code>PreconditionFailed</Code><Message>exists</Message></Error>`))
			return
		}
	}
	c.next.ServeHTTP(w, r)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:9000", "listen address")
	bucket := flag.String("bucket", "hop-demo", "bucket to create")
	flag.Parse()
	backend := s3mem.New()
	if err := backend.CreateBucket(*bucket); err != nil {
		log.Fatal(err)
	}
	log.Printf("fake S3 on http://%s (bucket %s)", *addr, *bucket)
	log.Fatal(http.ListenAndServe(*addr, conditionalWrites{next: gofakes3.New(backend).Server(), backend: backend}))
}
