package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

//http server to send files

var ZipMap map[string]string
var ZipMapFile = "zipmap.dat"

func serve() {
	ZipMap = make(map[string]string)
	restoreZipMap()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// remove .zip from the name
		zip := strings.ReplaceAll(path.Base(r.URL.Path), ".zip", "")
		http.ServeFile(w, r, ZipMap[zip])
	})
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// adapted from https://stackoverflow.com/questions/19762413/how-to-serialize-deserialize-a-map-in-go
func storeZipMap() {
	b := new(bytes.Buffer)
	e := gob.NewEncoder(b)
	// Encoding the map
	err := e.Encode(ZipMap)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = os.WriteFile(ZipMapFile, b.Bytes(), 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func restoreZipMap() {
	rb, err := os.ReadFile(ZipMapFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	b := bytes.NewBuffer(rb)
	var decodedMap map[string]int
	d := gob.NewDecoder(b)

	// Decoding the serialized data
	err = d.Decode(&decodedMap)
	if err != nil {
		fmt.Println(err)
		return
	}
}
