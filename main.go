package main

import (
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
	"strings"

	"github.com/livebud/cli"
	"github.com/livebud/watcher"
	"github.com/matthewmueller/dev/internal/graceful"
	"github.com/matthewmueller/dev/internal/hot"
	"github.com/matthewmueller/dev/internal/pubsub"
	"github.com/matthewmueller/dev/internal/sh"
	"github.com/matthewmueller/dev/internal/virtual"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cli := cli.New("dev", "personal dev tooling")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	{ // serve [flags] [dir]
		cmd := new(Serve)
		cli := cli.Command("serve", "serve a directory")
		cli.Flag("listen", "address to listen on").String(&cmd.Listen).Default(":3000")
		cli.Flag("live", "enable live reloading").Bool(&cmd.Live).Default(true)
		cli.Flag("open", "open browser").Bool(&cmd.Browser).Default(true)
		cli.Arg("dir").String(&cmd.Dir).Default(".")
		cli.Run(cmd.Run)
	}

	{ // watch [flags] [dir]
		cmd := new(Watch)
		cli := cli.Command("watch", "watch a directory")
		cli.Flag("ignore", "ignore files matching pattern").Strings(&cmd.Ignore).Default()
		cli.Flag("clear", "clear screen every change").Bool(&cmd.Clear).Default(true)
		cli.Arg("command").String(&cmd.Command)
		cli.Args("args").Strings(&cmd.Args).Default()
		cli.Run(cmd.Run)
	}

	return cli.Parse(ctx, os.Args[1:]...)
}

type Serve struct {
	Listen  string
	Live    bool
	Dir     string
	Browser bool
}

func (s *Serve) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)
	ps := pubsub.New()
	host, portStr, err := net.SplitHostPort(s.Listen)
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
	eg.Go(s.serve(ctx, ln, ps))
	if s.Live {
		eg.Go(s.watch(ctx, ps))
	}
	if s.Browser {
		if err := exec.CommandContext(ctx, "open", url).Run(); err != nil {
			return err
		}
	}
	return eg.Wait()
}

func (s *Serve) serve(ctx context.Context, ln net.Listener, ps pubsub.Subscriber) func() error {
	return func() error {
		fs := http.FileServer(http.FS(s))
		return graceful.Serve(ctx, ln, s.handler(hot.New(ps), fs))
	}
}

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

func (s *Serve) handler(live http.Handler, fs http.Handler) http.Handler {
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

func (s *Serve) Open(name string) (fs.File, error) {
	f, err := os.Open(filepath.Join(s.Dir, name))
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
	// Peak at the first 512 bytes of the file or the entire file
	// (whichever is smaller) and attempt to infer the content type
	// from its contents.
	firstBytes := make([]byte, 512)
	if _, err = io.ReadFull(f, firstBytes); err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	// Seek back to the start
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	contentType := http.DetectContentType(firstBytes)
	isHTML := isHTML(contentType)
	allAscii := allAscii(firstBytes)
	// If the content type isn't HTML, just return the file
	if !isHTML && !allAscii {
		return f, nil
	}
	// If we detect HTML, inject the live reload script
	html, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	// Close the existing file because we don't need it anymore
	if f.Close(); err != nil {
		return nil, err
	}
	// Inject the live reload script
	if isHTML {
		html = append(html, []byte(liveReloadScript)...)
	} else {
		html = []byte(fmt.Sprintf(htmlPage, liveReloadScript, string(html)))
	}
	// Create a buffered file
	bf := &virtual.File{
		Path:    name,
		Data:    html,
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
	}
	return bf.Open(), nil
}

func isHTML(contentType string) bool {
	return strings.Contains(contentType, "text/html")
}

func allAscii(b []byte) bool {
	for _, c := range b {
		if c > 127 {
			return false
		}
	}
	return true
}

func (s *Serve) watch(ctx context.Context, ps pubsub.Publisher) func() error {
	return func() error {
		return watcher.Watch(ctx, s.Dir, func(events []watcher.Event) error {
			if len(events) == 0 {
				return nil
			}
			event := events[0]
			ps.Publish(string(event.Op), []byte(event.Path))
			return nil
		})
	}
}

type Watch struct {
	Ignore  []string
	Clear   bool
	Command string
	Args    []string
}

func (w *Watch) Run(ctx context.Context) error {
	if w.Clear {
		clear()
	}
	// Run initially
	cmd := sh.Command{
		Stderr: os.Stderr,
		Stdout: os.Stdout,
		Stdin:  os.Stdin,
		Env:    os.Environ(),
		Dir:    ".",
	}
	if err := cmd.Start(ctx, w.Command, w.Args...); err != nil {
		// Don't exit on errors
		fmt.Fprintln(os.Stderr, err)
	}
	// Watch for changes
	return watcher.Watch(ctx, ".", func(events []watcher.Event) error {
		if len(events) == 0 {
			return nil
		}
		if w.Clear {
			clear()
		}
		if err := cmd.Restart(ctx); err != nil {
			// Don't exit on errors
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	})
}

func clear() {
	fmt.Fprint(os.Stdout, "\033[H\033[2J")
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
