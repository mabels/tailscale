// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package logheap logs a heap pprof profile.
package logheap

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"runtime"
	"runtime/pprof"
	"time"
)

// LogHeap writes a JSON logtail record with the base64 heap pprof to
// os.Stderr.
func LogHeap(postURL string) {
	if postURL == "" {
		return
	}
	runtime.GC()
	buf := new(bytes.Buffer)
	pprof.WriteHeapProfile(buf)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", postURL, buf)
	if err != nil {
		log.Printf("LogHeap: %v", err)
		return
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("LogHeap: %v", err)
		return
	}
	defer res.Body.Close()
}
