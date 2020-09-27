package flash

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Message struct {
	Category string `json:"c"`
	Text     string `json:"t"`
}

// Push writes a single flash message. This function must be called before the
// HTTP response header is written.
func Push(w http.ResponseWriter, category, text string) {
	raw, _ := json.Marshal(Message{Category: category, Text: text})
	now := time.Now()
	http.SetCookie(w, &http.Cookie{
		Name:     fmt.Sprintf("flash_%d", now.UnixNano()),
		Value:    base64.StdEncoding.EncodeToString(raw),
		HttpOnly: true,
		Expires:  time.Now().Add(time.Hour),
	})
}

// PopAll returns all flash messages and deletes them from the cookie. This
// function must be called before the HTTP response header is written.
func PopAll(w http.ResponseWriter, r *http.Request) []*Message {
	var flashes []*http.Cookie
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "flash_") {
			flashes = append(flashes, c)
		}
	}

	sort.Slice(flashes, func(i, j int) bool {
		return flashes[i].Name < flashes[j].Name
	})

	msgs := make([]*Message, 0, len(flashes))
	for _, c := range flashes {
		http.SetCookie(w, &http.Cookie{
			Name:     c.Name,
			MaxAge:   -1,
			Expires:  time.Unix(1, 0),
			HttpOnly: c.HttpOnly,
		})
		raw, err := base64.StdEncoding.DecodeString(c.Value)
		if err != nil {
			continue
		}
		var m Message
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		msgs = append(msgs, &m)
	}
	return msgs
}

// Embed returns an HTTP middleware that extends decorated HTTP handler
// response with flash messages.
//
// A template can be provided to render flash Message using a custom template.
// If nil template is given, a default template is used.
//
// If the request response body contains <flashmessages> tag, that tag is
// replaced with flash messages (or removed). If <flashmessages> tag is not
// present, flash messages are inserted before </body>.
//
// Search algorithm scans data passed in the ResponseWriter Write calls. For
// this reason, the response body is expected to make Write calls with payload
// containing complete tags, for example []byte("<div>foo") and not
// []byte("<di") or []byte("</s")))
func Embed(tmpl *template.Template) func(http.Handler) http.Handler {
	if tmpl == nil {
		tmpl = defaultTmpl
	}
	return func(next http.Handler) http.Handler {
		return &messageMiddleware{
			tmpl: tmpl,
			next: next,
		}
	}
}

var defaultTmpl = template.Must(template.New("").Parse(`
<div class="flash-messages">
	{{- range . -}}
		<div class="alert alert-{{.Category}}">{{.Text}}</div>
	{{- end -}}
</div>
`))

type messageMiddleware struct {
	tmpl *template.Template
	next http.Handler
}

func (m messageMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fe := flashEmbedder{
		w:    w,
		r:    r,
		tmpl: m.tmpl,
	}
	m.next.ServeHTTP(&fe, r)
}

type flashEmbedder struct {
	embed *bool
	w     http.ResponseWriter
	r     *http.Request
	msgs  []*Message
	tmpl  *template.Template
}

func (f *flashEmbedder) Header() http.Header {
	return f.w.Header()
}

func (f *flashEmbedder) Write(data []byte) (int, error) {
	if f.embed == nil {
		ct := f.w.Header().Get("content-type")
		if ct == "" {
			ct = http.DetectContentType(data)
		}
		isHTML := strings.HasPrefix(ct, "text/html")
		f.embed = &isHTML
		if isHTML {
			f.msgs = PopAll(f.w, f.r)
		}
	}

	if !*f.embed {
		return f.w.Write(data)
	}

	start := bytes.Index(data, []byte(`<flashmessages>`))
	end := start + len("<flashmessages>")
	if start < 0 && len(f.msgs) > 0 {
		start = bytes.Index(data, []byte(`</body>`))
		end = start
	}

	if start < 0 {
		return f.w.Write(data)
	}

	total, err := f.w.Write(data[:start])
	if err != nil {
		return total, err
	}

	if len(f.msgs) > 0 {
		n, err := f.w.Write(f.renderFlash())
		total += n
		if err != nil {
			return total, err
		}
		f.msgs = nil
	}

	n, err := f.w.Write(data[end:])
	total += n
	return total, err
}

func (f *flashEmbedder) WriteHeader(statusCode int) {
	f.w.WriteHeader(statusCode)
}

func (f *flashEmbedder) renderFlash() []byte {
	var b bytes.Buffer
	_ = f.tmpl.Execute(&b, f.msgs)
	return b.Bytes()
}
