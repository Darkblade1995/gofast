package gofast

import (
	"encoding/json"
	"log"
	"net/http"
)

func (r *Router) ServeOpenAPI(path, title, version string) {
	r.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, req *http.Request) {
		spec := GenerateOpenAPI(title, version, r.Routes())
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(spec); err != nil {
			log.Printf("[gofast] failed to encode OpenAPI spec: %v", err)
		}
	})
}

func (r *Router) ServeSwaggerUI(uiPath, specPath string) {
	html := swaggerUIHTML(specPath)

	r.mux.HandleFunc("GET "+uiPath, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			log.Printf("[gofast] failed to write Swagger UI HTML: %v", err)
		}
	})
}

func swaggerUIHTML(specPath string) string {
	return `<!DOCTYPE html>
<html>
<head>
  <title>GoFast API Docs</title>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui.min.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.11.0/swagger-ui-bundle.min.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "` + specPath + `",
        dom_id: "#swagger-ui"
      });
    };
  </script>
</body>
</html>`
}
