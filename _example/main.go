package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/husio/flash"
)

func main() {
	tmpl, _ := template.New("").Funcs(template.FuncMap{
		"flashmessages": func() []*flash.Message { return nil },
	}).Parse(`
<!doctype html>
<html>
<style>
label { display: block; }
.flash-messages { position: absolute; top: 2em; right: 2em; }
.alert { margin: 1em; background: #FFF6D1; padding: 1.4em; min-width: 10em; border-radius: 5px; font-size: 1.4em; }
.alert-info { background: #BCEFFF; }
.alert-error { background: #F7C5B7; }
</style>
<body>

<flashmessages>

<form action="." method="POST">
	<label>
		Category:
		<input type="text" name="category" value="info" required>
	</label>
	<label>
		Content. Messages are split by the newline character:
		<textarea name="text"></textarea>
	</label>
	<button>Submit</button>
</form>
</body>
</html>
	`)

	mux := http.NewServeMux()
	mux.Handle("/", &flashDemo{tmpl: tmpl})

	app := flash.Embed(nil)(mux)

	fmt.Println("HTTP server listening on port 8000")
	http.ListenAndServe(":8000", app)
}

type flashDemo struct {
	tmpl *template.Template
}

func (f *flashDemo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, msg := range strings.Split(r.Form.Get("text"), "\n") {
			msg = strings.TrimSpace(msg)
			if len(msg) == 0 {
				continue
			}
			flash.Push(w, r.Form.Get("category"), msg)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := f.tmpl.Execute(w, nil); err != nil {
		log.Printf("cannot render template: %s", err)
	}
}
