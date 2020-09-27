package flash

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware(t *testing.T) {
	cases := map[string]struct {
		Body     string
		Messages []Message
		WantBody string
	}{
		"no flash messages": {
			Body:     `<!doctype html><body></body>`,
			Messages: []Message{},
			WantBody: `<!doctype html><body></body>`,
		},
		"not an HTML document does not pop messages": {
			Body: `{"content":"<!doctype html><body></body>"}`,
			Messages: []Message{
				{Category: "a-category", Text: "a-text"},
			},
			WantBody: `{"content":"<!doctype html><body></body>"}`,
		},
		"a single message without a custom placement tag": {
			Body: `<!doctype html><body></body>`,
			Messages: []Message{
				{Category: "a-category", Text: "a-text"},
			},
			WantBody: "<!doctype html><body>[a-category:a-text]</body>",
		},
		"a single message with a custom placement tag": {
			Body: `<!doctype html><body><span><flashmessages></span></body>`,
			Messages: []Message{
				{Category: "a-category", Text: "a-text"},
			},
			WantBody: "<!doctype html><body><span>[a-category:a-text]</span></body>",
		},
		"no message with a custom placement tag": {
			Body:     `<!doctype html><body><span><flashmessages></span></body>`,
			Messages: []Message{},
			WantBody: "<!doctype html><body><span></span></body>",
		},
		"multiple messages with a custom placement tag": {
			Body: `<!doctype html><body><span><flashmessages></span></body>`,
			Messages: []Message{
				{Category: "a", Text: "A"},
				{Category: "b", Text: "B"},
				{Category: "c", Text: "C"},
			},
			WantBody: "<!doctype html><body><span>[a:A][b:B][c:C]</span></body>",
		},
		"multiple messages without a custom placement tag": {
			Body: `<!doctype html><body><span></span></body>`,
			Messages: []Message{
				{Category: "a", Text: "A"},
				{Category: "b", Text: "B"},
				{Category: "c", Text: "C"},
			},
			WantBody: "<!doctype html><body><span></span>[a:A][b:B][c:C]</body>",
		},
	}
	for testName, tc := range cases {
		t.Run(testName, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Body is written in multiple calls, using
				// random chunk sizes, to ensure that the parse
				// does not rely on a complete document being
				// sent.
				//body := tc.Body
				//for len(body) > 0 {
				//	n := rand.Intn(len(body) + 1)
				//	_, _ = io.WriteString(w, body[:n])
				//	body = body[n:]
				//}
				io.WriteString(w, tc.Body)
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			for i, m := range tc.Messages {
				raw, err := json.Marshal(m)
				if err != nil {
					t.Fatalf("cannot marshal message: %s", err)
				}
				r.AddCookie(&http.Cookie{
					Name:  fmt.Sprintf("flash_%d", i),
					Value: base64.StdEncoding.EncodeToString(raw),
				})
			}

			tmpl := template.Must(template.New("").Parse(`
			{{- range . -}}
				[{{.Category}}:{{.Text}}]
			{{- end -}}
			`))

			app := Embed(tmpl)(handler)
			app.ServeHTTP(w, r)

			if body := w.Body.String(); body != tc.WantBody {
				t.Fatalf("want %s\n got: %s", tc.WantBody, body)
			}
		})
	}
}

func flash(category, text string) string {
	w := httptest.NewRecorder()
	Push(w, category, text)
	return w.Header()["Set-Cookie"][0]
}
