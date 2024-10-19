package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
    "strings"
)

type apiConfig struct{
    serverHits atomic.Int32
}


func (cfg *apiConfig) resetMetricHandler(writer http.ResponseWriter, req *http.Request){
    req.Header.Set("Content-Type", "text/plain; charset=utf-8")
    writer.WriteHeader(200)
    cfg.serverHits.Store(0) 
    fmt.Println("Reset metrics called")

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){

        cfg.serverHits.Add(1)
        next.ServeHTTP(w,r)
    })

}

func (cfg *apiConfig) hitsMetricHandler(writer http.ResponseWriter, req *http.Request){
    req.Header.Set("Content-Type", "text/html")
    writer.WriteHeader(200)
    writer.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %v times!</p>
  </body>
</html>`, cfg.serverHits.Load())))

}

func validateChirpHandler(writer http.ResponseWriter, req *http.Request){
   type chirpBody struct{
        Body string `json:"body"`
    }

    type validResponse struct{
        Valid bool `json:"valid"`
    }

    type cleanedResponse struct{
        Cleaned_body string `json:"cleaned_body"`
    }

    decoder := json.NewDecoder(req.Body)
    body := chirpBody{}
    if err := decoder.Decode(&body); err != nil{
        respondWithError(writer, 500, "Something went wrong")
        return

    }

    if len(body.Body) > 140{
        respondWithError(writer, 400, "Chirp is too long")
        return
    }
    //words
    words := strings.Split(body.Body, " ")
    var arr []string
    for _, word := range words{
        switch strings.ToLower(word){
        case "kerfuffle":
            fallthrough
        case "sharbert":
            fallthrough
        case "fornax":
            arr = append(arr, "****")
        default:
            arr = append(arr, word)
        }
    }
    cleaned_s := strings.Join(arr, " ")
    res := cleanedResponse{Cleaned_body: cleaned_s}
    respondWithJSON(writer, 200, res)

}
func respondWithError(w http.ResponseWriter, code int, msg string){
    type errorResponse struct{
        Error string `json:"error"`
    }
    w.WriteHeader(code)
    w.Header().Set("Content-Type", "application/json")
    res := errorResponse{Error:msg}
    dat, err := json.Marshal(res)
    if err == nil{
        w.Write(dat)
    }

}
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}){
    w.WriteHeader(code)
    w.Header().Set("Content-Type", "application/json")
    dat, err := json.Marshal(payload)
    if err == nil{
        w.Write(dat)
    }
}
func readinessHandler(writer http.ResponseWriter, req *http.Request){
    req.Header.Set("Content-Type", "text/plain; charset=utf-8")
    writer.WriteHeader(200)
    writer.Write([]byte("OK"))
}


func main(){
    fpRoot := "."
    apiCfg := &apiConfig{}
    mux := http.NewServeMux()
    // readiness 
    mux.HandleFunc("GET /api/healthz", readinessHandler)
    //metric
    mux.HandleFunc("GET /admin/metrics", apiCfg.hitsMetricHandler)
    //reset
    mux.HandleFunc("POST /admin/reset", apiCfg.resetMetricHandler)
    //validate chirp
    mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
    mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(fpRoot)))))
    server := &http.Server{Addr:":8080" ,Handler:mux}
    fmt.Println("Server started and running...")
    err := server.ListenAndServe()
    if err != nil{
        fmt.Errorf("error was reported %v", err)
    }

}
