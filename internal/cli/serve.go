package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/livebud/watcher"
	"github.com/matthewmueller/dev/internal/graceful"
	"github.com/matthewmueller/dev/internal/hot"
	"github.com/matthewmueller/dev/internal/pubsub"
	"github.com/matthewmueller/virt"
	"golang.org/x/sync/errgroup"
)

// Minimal favicon
var favicon = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49,
	0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02,
	0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00, 0x01, 0x73, 0x52,
	0x47, 0x42, 0x00, 0xae, 0xce, 0x1c, 0xe9, 0x00, 0x00, 0x00, 0x04, 0x67, 0x41,
	0x4d, 0x41, 0x00, 0x00, 0xb1, 0x8f, 0x0b, 0xfc, 0x61, 0x05, 0x00, 0x00, 0x00,
	0x09, 0x70, 0x48, 0x59, 0x73, 0x00, 0x00, 0x0e, 0xc4, 0x00, 0x00, 0x0e, 0xc4,
	0x01, 0x95, 0x2b, 0x0e, 0x1b, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54,
	0x08, 0xd7, 0x63, 0xf8,
}

const liveReloadScript = `
<script>
var es = new EventSource('/.live');
es.onmessage = function(e) { window.location.reload(); }
</script>
`

const htmlPage = `<!doctype html>
<html>
<head>
	<meta charset="utf-8">
</head>
<body>
	%s
	%s
</body>
</html>
`

type Serve struct {
	Listen  string
	Live    bool
	Dir     string
	Browser bool
}

func (c *CLI) Serve(ctx context.Context, in *Serve) error {
	eg, ctx := errgroup.WithContext(ctx)
	ps := pubsub.New()
	host, portStr, err := net.SplitHostPort(in.Listen)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}
	// Find the next available port
	ln, port, err := findNextPort(host, port)
	if err != nil {
		return err
	}
	url := formatAddr(host, port)
	fmt.Println("Listening on", url)
	eg.Go(c.serveDir(ctx, ln, ps, in.Dir))
	if in.Live {
		eg.Go(c.watchDir(ctx, ps, in.Dir))
	}
	if in.Browser {
		if err := exec.CommandContext(ctx, "open", url).Run(); err != nil {
			return err
		}
	}
	return eg.Wait()
}

func (c *CLI) serveDir(ctx context.Context, ln net.Listener, ps pubsub.Subscriber, dir string) func() error {
	return func() error {
		fs := http.FileServer(http.FS(dirFS(dir)))
		return graceful.Serve(ctx, ln, handler(hot.New(ps), fs))
	}
}

func handler(live http.Handler, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/favicon.ico":
			w.Header().Set("Content-Type", "image/png")
			w.Write(favicon)
		case "/.live":
			live.ServeHTTP(w, r)
		default:
			w.Header().Set("Cache-Control", "no-store")
			fs.ServeHTTP(w, r)
		}
	})
}

func (c *CLI) watchDir(ctx context.Context, ps pubsub.Publisher, dir string) func() error {
	return func() error {
		return watcher.Watch(ctx, dir, func(events []watcher.Event) error {
			if len(events) == 0 {
				return nil
			}
			event := events[0]
			ps.Publish(string(event.Op), []byte(event.Path))
			return nil
		})
	}
}

type dirFS string

func (d dirFS) Open(name string) (fs.File, error) {
	dir := string(d)
	if filepath.Ext(name) != ".html" {
		return os.Open(filepath.Join(dir, name))
	}
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	// Skip directories
	if fi.IsDir() {
		return f, nil
	}
	defer f.Close()
	// If we detect HTML, inject the live reload script
	html, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	// Close the existing file because we don't need it anymore
	if err := f.Close(); err != nil {
		return nil, err
	}
	// Inject the live reload script
	if bytes.Contains(html, []byte("<html")) {
		html = append(html, []byte(liveReloadScript)...)
	} else {
		html = []byte(fmt.Sprintf(htmlPage, liveReloadScript, string(html)))
	}
	// Create a buffered file
	bf := &virt.File{
		Path:    name,
		Data:    html,
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
	}
	return virt.To(bf), nil
}

// Find the next available port starting at 3000
func findNextPort(host string, port int) (net.Listener, int, error) {
	for i := 0; i < 100; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port+i))
		if err == nil {
			return ln, port + i, nil
		}
	}
	return nil, 0, fmt.Errorf("could not find an available port")
}

func formatAddr(host string, port int) string {
	if host == "" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}
