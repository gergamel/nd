package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"errors"
	"strings"
	"bytes"
	"time"
	"log"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
)

type ObjectStore interface {
	List() ([]string, error)
	Exists(hash string) bool
	Get(hash string, fromByte int64) (io.ReadCloser, error)
	Put(hash string, f io.Reader) (int64, error)
	DetectContentType(hash string) string
}

type MetaData struct {
	FileName	string	`json:"filename"`
	ContentType	string	`json:"content-type"`
	Length		int64	`json:"size"`
	Created		int64	`json:"created"`
}

type ResponseData struct {
	code	int
	Status	string		`json:"status"`
	Oid	string		`json:"oid,omitempty"`
	Meta	*MetaData	`json:"meta,omitempty"`
}

type MetaStore interface {
	Get(hash string) (*MetaData, error)
	Put(hash string, d *MetaData) error
}

// App links a Router, ObjectStore, and MetaStore to provide the LFS server.
type App struct {
	router		*mux.Router
	objectStore	ObjectStore
	metaStore	MetaStore
}

func NewApp(st ObjectStore, mst MetaStore) *App {
	app := &App{objectStore: st, metaStore: mst}
	r := mux.NewRouter()
	
	r.HandleFunc("/", app.RootHandler).Methods("GET")
	
	r.HandleFunc("/objects", app.DirHandler).Methods("GET")
	r.HandleFunc("/objects/", app.DirHandler).Methods("GET")
	
	r.HandleFunc("/meta/{hash}", app.GetMetaHandler).Methods("GET")//.MatcherFunc(MetaMatcher)
	
	r.HandleFunc("/objects/{hash}", app.GetHandler).Methods("GET", "HEAD")
	r.HandleFunc("/objects/{hash}", app.PutHandler).Methods("PUT")//.MatcherFunc(ContentMatcher)
	
	app.router = r

	return app
}

// App implements ServeHTTP so it is an http.Handler
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err == nil {
		context.Set(r, "RequestID", fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
	}

	a.router.ServeHTTP(w, r)
}

// Serve calls http.Serve with the provided Listener and the app's router
func (a *App) Serve(l net.Listener) error {
	return http.Serve(l, a)
}

func writeResponseData(w http.ResponseWriter, r *http.Request, d ResponseData) {
	w.Header().Set("Content-Type", metaMediaType)
	w.WriteHeader(d.code)
	enc := json.NewEncoder(w)
	enc.Encode(d)
}

func writeError(w http.ResponseWriter, r *http.Request, code int, e error) {
	d := ResponseData{code: code, Status: e.Error(), Meta: nil}
	writeResponseData(w, r, d)
}

func (a *App) RootHandler(w http.ResponseWriter, r *http.Request) {
	d := ResponseData{code: 200, Status: "OK", Meta: nil}
	writeResponseData(w, r, d)
}

func (a *App) DirHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", metaMediaType)
	w.WriteHeader(200)
	objs, err := a.objectStore.List()
	if ((err != nil) || (len(objs) == 0)) {
		fmt.Fprintf(w, `{"objects":[]}`)
		log.Printf("Return empty list: %s\n", err)
		return
	}
	
	fmt.Fprint(w, "{\"objects\": ")
	d := '['
	for _, h := range objs {
		fmt.Fprintf(w, "%c\"%s\"", d, h)
		d = ','
	}
	fmt.Fprint(w, "]}")
	log.Printf("Return list of hashes\n")
}

func (a *App) GetMetaHandler(w http.ResponseWriter, r *http.Request) {
	mv := mux.Vars(r)
	oid := mv["hash"]
	meta,err := a.metaStore.Get(oid)
	if err != nil {
		writeError(w, r, 404, err)
		return
	}
	log.Printf("Content-Disposition: attachment; filename=%s", meta.FileName)
	log.Printf("Content-Type: %s", meta.ContentType)
	log.Printf("Content-Length: %d", meta.Length)

	d := ResponseData{code: 200, Status: "OK", Oid: oid, Meta: meta}
	writeResponseData(w, r, d)
}

func (a *App) GetHandler(w http.ResponseWriter, r *http.Request) {
	mv := mux.Vars(r)
	oid := mv["hash"]
	log.Printf("GetHandler() object %s\n", oid)
	
	/* TODO: Support resume download using Range header
	var fromByte int64
	statusCode := 200
	if rangeHdr := r.Header.Get("Range"); rangeHdr != "" {
		regex := regexp.MustCompile(`bytes=(\d+)\-.*`)
		match := regex.FindStringSubmatch(rangeHdr)
		if match != nil && len(match) > 1 {
			statusCode = 206
			fromByte, _ = strconv.ParseInt(match[1], 10, 32)
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", fromByte, meta.Size-1, int64(meta.Size)-fromByte))
		}
	}
	content, err := a.contentStore.Get(meta, fromByte)*/

	meta,err := a.metaStore.Get(oid)
	if err != nil {
		writeError(w, r, 404, err)
		return
	}
	log.Printf("Content-Disposition: attachment; filename=%s", meta.FileName)
	log.Printf("Content-Type: %s", meta.ContentType)
	log.Printf("Content-Length: %d", meta.Length)
	
	content, err := a.objectStore.Get(oid, 0)
	if err != nil {
		writeError(w, r, 404, err)
		return
	}
	defer content.Close()
	
	/* Also need to properly pass the accept content-type header in the request */
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", meta.FileName))
	w.Header().Set("Content-Type", meta.ContentType)
	w.WriteHeader(200)
	io.Copy(w, content)
}

func (a *App) PutHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("================================================================================\n")
	mv := mux.Vars(r)
	oid := mv["hash"]
	log.Printf("PUT object %s\n", oid)
	if a.objectStore.Exists(oid) {
		writeError(w, r, 409, errors.New("Exists"))
		log.Print("Rejected: Already exists\n")
		return
	}
	
	// Set up for Multipart streaming read
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, r, 500, err)
		log.Printf("Failed: %s\n", err)
		return
	}
	
	meta := MetaData{FileName: "", ContentType: "", Length: 0}
	ct := "application/octet-stream"
	
	// Iterate through the parts
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		log.Printf("FormName: %s\n", part.FormName())
		log.Printf("FileName: %s\n", part.FileName())
		meta.FileName = part.FileName()
		for key, value := range part.Header {
			log.Printf("%s: %s\n", key, value[0])
			if (key == "Content-Type") {
				ct = value[0]
			}
		}
		
		// if part.FileName() is empty, decode the value
		if part.FileName() == "" {
			buf := new(bytes.Buffer)
			buf.ReadFrom(part)
			log.Printf("Value: %s\n", buf.String())
			continue
		}
		
		// Otherwise, part is a file, try to put it into the store
		written, err := a.objectStore.Put(oid, part)
		if err != nil {
			writeError(w, r, 500, err)
			log.Printf("Failed: %s\n", err)
			return
		}
		meta.Length = written
		meta.ContentType = a.objectStore.DetectContentType(oid)
		if (ct != meta.ContentType) {
			log.Printf("Detected Content-Type: %s", meta.ContentType)
		}
		meta.Created = time.Now().Unix()
		err = a.metaStore.Put(oid, &meta)
		if err != nil {
			writeError(w, r, 500, err)
			log.Printf("Failed: %s\n", err)
			return
		}
		d := ResponseData{code: 201, Status: "OK", Oid: oid, Meta: &meta}
		writeResponseData(w, r, d)
		return
	}
	writeError(w, r, 400, errors.New("No file parts found in request"))
}

// ContentMatcher provides a mux.MatcherFunc that only allows requests that contain
// an Accept header with the contentMediaType
func ContentMatcher(r *http.Request, m *mux.RouteMatch) bool {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	return mt == contentMediaType
}

// MetaMatcher provides a mux.MatcherFunc that only allows requests that contain
// an Accept header with the metaMediaType
func MetaMatcher(r *http.Request, m *mux.RouteMatch) bool {
	mediaParts := strings.Split(r.Header.Get("Accept"), ";")
	mt := mediaParts[0]
	return mt == metaMediaType
}

func logRequest(r *http.Request, status int) {
	logger.Log(kv{"method": r.Method, "url": r.URL, "status": status, "request_id": context.Get(r, "RequestID")})
}
