package web

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"
)

//go:embed templates/*.html templates/partials/*.html templates/alerts/*.html templates/admin/*.html templates/filters/*.html
var templateFS embed.FS

//go:embed static/style.css
var staticCSS []byte

var funcMap = template.FuncMap{
	"truncate": func(s string, n int) string {
		runes := []rune(s)
		if len(runes) <= n {
			return s
		}
		return string(runes[:n]) + "…"
	},
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"derefOr": func(s *string, fallback string) string {
		if s == nil || *s == "" {
			return fallback
		}
		return *s
	},
	"activeLabel": func(active int) string {
		if active == 1 {
			return "Active"
		}
		return "Inactive"
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"seq": func(n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = i + 1
		}
		return s
	},
	"commaFmt": func(n int64) string {
		s := fmt.Sprintf("%d", n)
		if len(s) <= 3 {
			return s
		}
		var result []byte
		for i, c := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				result = append(result, ',')
			}
			result = append(result, byte(c))
		}
		return string(result)
	},
	"containsCSV": func(csv string, val string) bool {
		for _, v := range strings.Split(csv, ",") {
			if strings.TrimSpace(v) == val {
				return true
			}
		}
		return false
	},
	"stripHTML":    stripHTML,
	"naicsLabel":   naicsLabel,
	"setAsideDesc": setAsideDesc,
	"oppTypeDesc":  oppTypeDesc,
	"boolChecked": func(b bool) template.HTMLAttr {
		if b {
			return "checked"
		}
		return ""
	},
	"pageCount": func(total int64, limit int) int {
		if limit <= 0 {
			return 1
		}
		pages := int(total) / limit
		if int(total)%limit != 0 {
			pages++
		}
		return pages
	},
	"currentPage": func(offset, limit int) int {
		if limit <= 0 {
			return 1
		}
		return offset/limit + 1
	},
}

func loadTemplates() map[string]*template.Template {
	pages := map[string]*template.Template{}

	shared := []string{
		"templates/layout.html",
		"templates/partials/results.html",
		"templates/partials/pagination.html",
	}

	pageFiles := map[string][]string{
		"login.html":         {"templates/login.html"},
		"opportunities.html": {"templates/opportunities.html"},
		"opportunity.html":   {"templates/opportunity.html"},
		"results.html":       {"templates/partials/results.html", "templates/partials/pagination.html"},
		"alerts_list.html":   {"templates/alerts/list.html"},
		"alerts_form.html":   {"templates/alerts/form.html"},
		"alerts_detail.html": {"templates/alerts/detail.html"},
		"admin_sync.html":    {"templates/admin/sync.html"},
		"admin_users.html":   {"templates/admin/users.html"},
		"filters_list.html":  {"templates/filters/list.html"},
		"filters_edit.html":  {"templates/filters/edit.html"},
	}

	for name, files := range pageFiles {
		allFiles := make([]string, 0, len(shared)+len(files))
		allFiles = append(allFiles, shared...)
		allFiles = append(allFiles, files...)
		tmpl := template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS, allFiles...),
		)
		pages[name] = tmpl
	}

	return pages
}

func stripHTML(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			buf.WriteRune(' ')
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func loadTemplatesFromDisk() (map[string]*template.Template, error) {
	dir := "internal/web"
	pages := map[string]*template.Template{}

	shared := []string{
		filepath.Join(dir, "templates/layout.html"),
		filepath.Join(dir, "templates/partials/results.html"),
		filepath.Join(dir, "templates/partials/pagination.html"),
	}

	pageFiles := map[string][]string{
		"login.html":         {"templates/login.html"},
		"opportunities.html": {"templates/opportunities.html"},
		"opportunity.html":   {"templates/opportunity.html"},
		"results.html":       {"templates/partials/results.html", "templates/partials/pagination.html"},
		"alerts_list.html":   {"templates/alerts/list.html"},
		"alerts_form.html":   {"templates/alerts/form.html"},
		"alerts_detail.html": {"templates/alerts/detail.html"},
		"admin_sync.html":    {"templates/admin/sync.html"},
		"admin_users.html":   {"templates/admin/users.html"},
		"filters_list.html":  {"templates/filters/list.html"},
		"filters_edit.html":  {"templates/filters/edit.html"},
	}

	for name, files := range pageFiles {
		allFiles := make([]string, 0, len(shared)+len(files))
		allFiles = append(allFiles, shared...)
		for _, f := range files {
			allFiles = append(allFiles, filepath.Join(dir, f))
		}
		tmpl, err := template.New("").Funcs(funcMap).ParseFiles(allFiles...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		pages[name] = tmpl
	}

	return pages, nil
}

func renderTemplate(w io.Writer, templates map[string]*template.Template, name string, data any) error {
	tmpl, ok := templates[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

func setUser(ctx context.Context, user *SessionUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func getUser(r interface{ Context() context.Context }) *SessionUser {
	if u, ok := r.Context().Value(userContextKey).(*SessionUser); ok {
		return u
	}
	return nil
}
