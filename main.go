package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/relios899/server/internal/database"
)

type chirpResponse struct {
    Id string `json:"id"`
    CreatedAt string `json:"created_at"`
    UpdatedAt string `json:"updated_at"`
    Body string `json:"body"`
    UserID string `json:"user_id"`
}

type apiConfig struct{
    serverHits atomic.Int32
    db *database.Queries
    platform string
}


func (cfg *apiConfig) resetMetricHandler(writer http.ResponseWriter, req *http.Request){
    if cfg.platform != "dev"{
        writer.WriteHeader(403)
        return
    }
    
    err := cfg.db.DeleteUsers(req.Context())
    if err != nil{
        respondWithError(writer, 500, "error deleting users")
        return
    }
    writer.WriteHeader(200)
    

    // req.Header.Set("Content-Type", "text/plain; charset=utf-8")
    // writer.WriteHeader(200)
    // cfg.serverHits.Store(0) 
    // fmt.Println("Reset metrics called")

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

func (cfg *apiConfig) chirpHandler(writer http.ResponseWriter, req *http.Request){
   type chirpBody struct{
        Body string `json:"body"`
        UserID uuid.NullUUID `json:"user_id"`
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
    // new logic to save chirp
    chirpData := database.CreateChirpParams{
        Body: cleaned_s,
        UserID: body.UserID,
    }

    chirp, err := cfg.db.CreateChirp(req.Context(), chirpData)
    if err != nil{
        respondWithError(writer, 500, "issue creating chirp")
        return
    }
    res := chirpResponse{
        Id: chirp.ID.String(),
        CreatedAt: chirp.CreatedAt.String(),
        UpdatedAt: chirp.UpdatedAt.String(),
        Body: chirp.Body,
        UserID: chirp.UserID.UUID.String(),
    }


    respondWithJSON(writer, 201, res)

}
func (cfg *apiConfig) getChirpsHandler(w http.ResponseWriter, r *http.Request){
    data, err := cfg.db.GetChirps(r.Context())
    if err != nil{
        respondWithError(w, 500, "issue with retrieving chirps")
        return
    }

    var res []chirpResponse
    for _, item := range data{
        val := chirpResponse{
            Id: item.ID.String(),
            CreatedAt: item.CreatedAt.String(),
            UpdatedAt: item.UpdatedAt.String(),
            Body: item.Body,
            UserID: item.UserID.UUID.String(),
        }
        res = append(res, val)
    }
    respondWithJSON(w, 200, res)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request){
    chirpID, err := uuid.Parse(r.PathValue("chirpID"))
    if err != nil{
        respondWithError(w, 500, "bad uuid")
        return
    }
    data, err := cfg.db.GetChirp(r.Context(), chirpID)
    if err != nil{
        respondWithError(w, 404, "data not found")
        return
    }
    val := chirpResponse{
        Id: data.ID.String(),
        CreatedAt: data.CreatedAt.String(),
        UpdatedAt: data.UpdatedAt.String(),
        Body: data.Body,
        UserID: data.UserID.UUID.String(),
    }

    respondWithJSON(w, 200, val)
}

func (cfg *apiConfig) createUsersHandler(w http.ResponseWriter, r *http.Request){
    type createUserBody struct{
        Email string `json:"email"`
    }

    decoder := json.NewDecoder(r.Body)
    body := createUserBody{}
    if err := decoder.Decode(&body); err != nil{
        respondWithError(w, 400, "problem with request body")
        return
    }
    user, err := cfg.db.CreateUser(r.Context(), body.Email)
    if err != nil{
        respondWithError(w, 400, "error with creating user")
    }
    type userCreatedResponse struct{
        Id string `json:"id"`
        CreatedAt string `json:"created_at"`
        UpdatedAt string `json:"updated_at"`
        Email string `json:"email"`
    }
    resData := userCreatedResponse{
        Id: user.ID.String(),
        CreatedAt: user.CreatedAt.String(),
        UpdatedAt: user.UpdatedAt.String(),
        Email: user.Email,
    }
    respondWithJSON(w, 201, resData)

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
    godotenv.Load()
    dbURL := os.Getenv("DB_URL")
    db, err := sql.Open("postgres", dbURL)
    fpRoot := "."
    apiCfg := &apiConfig{}
    apiCfg.db = database.New(db)
    apiCfg.platform = os.Getenv("PLATFORM")
    mux := http.NewServeMux()
    // readiness 
    mux.HandleFunc("GET /api/healthz", readinessHandler)
    //metric
    mux.HandleFunc("GET /admin/metrics", apiCfg.hitsMetricHandler)
    //reset
    mux.HandleFunc("POST /admin/reset", apiCfg.resetMetricHandler)
    //validate chirp
    mux.HandleFunc("POST /api/chirps", apiCfg.chirpHandler)
    //get chirps
    mux.HandleFunc("GET /api/chirps", apiCfg.getChirpsHandler)
    mux.HandleFunc("GET /api/chirps/{chirpID}" , apiCfg.getChirpHandler)
    mux.HandleFunc("POST /api/users", apiCfg.createUsersHandler)

    mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(fpRoot)))))
    server := &http.Server{Addr:":8080" ,Handler:mux}
    fmt.Println("Server started and running...")
    err = server.ListenAndServe()
    if err != nil{
        fmt.Errorf("error was reported %v", err)
    }

}
