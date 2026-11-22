package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"
)

func runServe(ctx context.Context, name, addr string, stdout io.Writer) int {
	catalog, err := loadCollection(name)
	if err != nil {
		fmt.Fprintf(stdout, "pgtmpl: load %q failed:\n%v\n", name, err)
		return 1
	}
	// Prepare every preview before listening. A failed collection or rendering
	// never publishes a partial preview or opens a server port.
	handler, err := newPreviewHandler(catalog, filepath.Base(name))
	if err != nil {
		fmt.Fprintln(stdout, "pgtmpl: preview:", err)
		return 1
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(stdout, "pgtmpl: serve:", err)
		return 1
	}
	defer listener.Close()
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second,
		ErrorLog: log.New(stdout, "pgtmpl: http: ", 0),
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				server.Close()
			}
		case <-done:
		}
	}()
	fmt.Fprintf(stdout, "pgtmpl: serving %q (%d templates) at http://%s/\nPress Ctrl+C to stop.\n", name, len(catalog.IDs()), listener.Addr())
	err = server.Serve(listener)
	close(done)
	<-stopped
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(stdout, "pgtmpl: serve:", err)
		return 1
	}
	return 0
}
