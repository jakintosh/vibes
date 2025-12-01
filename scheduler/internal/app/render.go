package app

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"

	db "event-scheduler/internal/db"
)

func render(w http.ResponseWriter, r *http.Request, tmplName string, data any) {
	// Check for HTMX header
	isHX := r.Header.Get("HX-Request") == "true"

	// Adjust path to templates since we are running from project root usually,
	// but let's verify where the binary runs.
	// Assuming the binary is run from the root of the repo or templates are in "templates/" relative to CWD.
	files := []string{"templates/" + tmplName}
	if !isHX {
		files = append(files, "templates/layout.template")
	}

	funcMap := template.FuncMap{
		"seq": func(start, end int) []int {
			var s []int
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
		"formatDate": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
		"statusClass": func(status db.EventStatus) string {
			return strings.ToLower(string(status))
		},
		"paymentClass": func(status db.PaymentStatus) string {
			return "payment-" + strings.ToLower(string(status))
		},
		"money": func(amount float64) string {
			return fmt.Sprintf("$%.2f", amount)
		},
		"amountDue": func(proposed, received float64) float64 {
			if proposed <= received {
				return 0
			}
			return proposed - received
		},
	}

	// Create a base template with functions
	// We use the name of the first file as the base name, which is standard for ParseFiles
	baseName := "layout.template"
	if isHX {
		baseName = tmplName
	}

	tmpl := template.New(baseName).Funcs(funcMap)
	var err error
	tmpl, err = tmpl.ParseFiles(files...)
	if err != nil {
		http.Error(w, "Template Parse Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isHX {
		err = tmpl.ExecuteTemplate(w, "content", data)
	} else {
		err = tmpl.ExecuteTemplate(w, "layout", data)
	}

	if err != nil {
		log.Println("Template execution error:", err)
	}
}
