package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
)

const dirName = "./uploads"

var dir = os.DirFS(dirName)

func genericFileAccess(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("name")
	log.Printf("Got %s request to file named %s\n", r.Method, filename)
	switch r.Method {
	case http.MethodGet:
		// Get logic
		if err := setHeaders(filename, w); err != nil {
			return
		}
		file, err := dir.Open(filename)
		defer file.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Got error while opening file named %s: %v\n", filename, err)
			return
		}
		_, err = io.Copy(w, file)
		if err != nil {
			log.Printf("Got error while copying: %v\n", err)
		}

	case http.MethodPut:
		// Put logic
		_, err := dir.(fs.StatFS).Stat(filename)
		var successCode int
		if os.IsNotExist(err) {
			successCode = http.StatusCreated
		} else {
			successCode = http.StatusNoContent
		}
		file, err := os.Create(path.Join(dirName, filename))
		defer file.Close()
		if err != nil {
			log.Printf("Unable to create new file: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, err = io.Copy(file, r.Body)
		if err != nil {
			log.Printf("Got error while copying: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(successCode)

	case http.MethodDelete:
		// Delete logic (wow, it's so simple)
		err := os.Remove(path.Join(dirName, filename))
		if err != nil {
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				log.Printf("Got error while deleting file on %s: %v\n", filename, err)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodHead:
		if err := setHeaders(filename, w); err == nil {
			w.WriteHeader(http.StatusOK)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func setHeaders(filename string, w http.ResponseWriter) error {
	stat, err := dir.(fs.StatFS).Stat(filename)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Got error while using stat %s: %v\n", filename, err)
		}
		return err
	}
	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(filename)))
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	return nil
}

type FileEntry struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Mimetype string `json:"mimetype"`
}

func allFiles(w http.ResponseWriter, req *http.Request) {
	entries, err := dir.(fs.ReadDirFS).ReadDir(".")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Got error while listing directory files: %v\n", err)
		return
	}
	files := make([]FileEntry, len(entries))
	for index, entry := range entries {
		file := &files[index]
		filename := entry.Name()
		file.Name = filename
		file.Mimetype = mime.TypeByExtension(path.Ext(filename))
		stat, err := entry.Info()
		if err != nil {
			log.Printf("Unable to get stats for file named %s: %v\n", filename, err)
			file.Size = -1
			continue
		}
		file.Size = stat.Size()
	}
	json_result, err := json.Marshal(files)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Unable to encode file entries to json: %v\n", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(json_result)
}

func main() {
	http.HandleFunc("/files/{name...}", genericFileAccess)
	http.HandleFunc("/files", allFiles)

	log.Fatal(http.ListenAndServe(":8000", nil))
}
